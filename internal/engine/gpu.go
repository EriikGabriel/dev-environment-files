package engine

import (
	"context"
	"sort"
	"strings"

	"github.com/EriikGabriel/dev-environment-files/internal/config"
)

// gpuScript lists the display adapters of a Windows machine, one per line.
const gpuScript = "Get-CimInstance Win32_VideoController | ForEach-Object { $_.Name + ' | ' + $_.AdapterCompatibility }"

// GPUVendor tells the kind of a graphics card from its name: nvidia, amd,
// intel, or "" for virtual and basic adapters.
func GPUVendor(name string) string {
	n := strings.ToLower(name)
	switch {
	case strings.Contains(n, "nvidia"), strings.Contains(n, "geforce"), strings.Contains(n, "quadro"):
		return "nvidia"
	case strings.Contains(n, "radeon"), strings.Contains(n, "amd"), strings.Contains(n, "advanced micro devices"):
		return "amd"
	case strings.Contains(n, "intel"):
		return "intel"
	}
	return ""
}

// GPUs lists the kinds of graphics card the machine has (nvidia, amd, intel),
// sorted; empty when they cannot be read (Linux, or the query failed).
func (e *Env) GPUs(ctx context.Context) []string {
	e.gpuMu.Lock()
	defer e.gpuMu.Unlock()
	if e.gpus != nil {
		return *e.gpus
	}
	found := e.CheckCache.lines("\x00gpus", func() []string { return e.readGPUs(ctx) })
	e.gpus = &found
	return found
}

func (e *Env) readGPUs(ctx context.Context) []string {
	found := []string{}
	if e.Info.OS == "windows" {
		if out, err := e.Runner.Output(ctx, e.script(gpuScript)...); err == nil {
			seen := map[string]bool{}
			for _, line := range strings.Split(out, "\n") {
				if v := GPUVendor(line); v != "" && !seen[v] {
					seen[v] = true
					found = append(found, v)
				}
			}
			sort.Strings(found)
		}
	}
	return found
}

var gpuNames = map[string]string{"nvidia": "NVIDIA", "amd": "AMD", "intel": "Intel"}

func gpuList(kinds []string) string {
	names := make([]string, len(kinds))
	for i, k := range kinds {
		if names[i] = gpuNames[strings.ToLower(k)]; names[i] == "" {
			names[i] = k
		}
	}
	return strings.Join(names, " ou ")
}

// Suggested tells whether an app should start selected on this machine:
// an app for a kind of graphics card (config.App.GPU) only when the machine
// has one, and an app for a device (config.App.Devices) only when it is
// connected. why explains the choice for those apps ("" for the others).
func (e *Env) Suggested(ctx context.Context, a config.App) (ok bool, why string) {
	if len(a.Devices) > 0 {
		return e.deviceSuggested(ctx, a.Devices)
	}
	if len(a.GPU) == 0 {
		return true, ""
	}
	have := e.GPUs(ctx)
	if len(have) == 0 {
		return true, "placa de vídeo não detectada"
	}
	for _, want := range a.GPU {
		for _, h := range have {
			if strings.EqualFold(want, h) {
				return true, "placa " + gpuList([]string{h}) + " detectada"
			}
		}
	}
	return false, "para placas " + gpuList(a.GPU) + "; esta máquina tem " + gpuList(have)
}

// devicesScript lists the connected devices, one per line: the instance id,
// the name Windows gives and, for USB devices, the name the device reports
// ("MCHOSE V9 PRO", "2.4G Wireless Mouse").
const devicesScript = "Get-PnpDevice -PresentOnly -ErrorAction SilentlyContinue | ForEach-Object { " +
	"$d = ''; if ($_.InstanceId.StartsWith('USB')) { $d = (Get-PnpDeviceProperty -InstanceId $_.InstanceId -KeyName 'DEVPKEY_Device_BusReportedDeviceDesc' -ErrorAction SilentlyContinue).Data }; " +
	"$_.InstanceId + ' | ' + $_.FriendlyName + ' | ' + $d }"

// Devices lists the connected devices (see devicesScript); empty when they
// cannot be read (Linux, or the query failed).
func (e *Env) Devices(ctx context.Context) []string {
	e.devicesMu.Lock()
	defer e.devicesMu.Unlock()
	if e.devices != nil {
		return *e.devices
	}
	found := e.CheckCache.lines("\x00devices", func() []string { return e.readDevices(ctx) })
	e.devices = &found
	return found
}

func (e *Env) readDevices(ctx context.Context) []string {
	found := []string{}
	if e.Info.OS == "windows" {
		if out, err := e.Runner.Output(ctx, e.script(devicesScript)...); err == nil {
			for _, line := range strings.Split(out, "\n") {
				if line = strings.TrimSpace(line); line != "" {
					found = append(found, line)
				}
			}
		}
	}
	return found
}

func (e *Env) deviceSuggested(ctx context.Context, want []string) (bool, string) {
	have := e.Devices(ctx)
	if len(have) == 0 {
		return true, "aparelhos não verificados"
	}
	for _, w := range want {
		for _, line := range have {
			if !strings.Contains(strings.ToLower(line), strings.ToLower(w)) {
				continue
			}
			// the name that matched (friendly or reported), else the id
			for _, part := range strings.Split(line, "|")[1:] {
				if part = strings.TrimSpace(part); strings.Contains(strings.ToLower(part), strings.ToLower(w)) {
					return true, "conectado: " + part
				}
			}
			return true, "conectado (" + w + ")"
		}
	}
	return false, "o aparelho não está conectado"
}

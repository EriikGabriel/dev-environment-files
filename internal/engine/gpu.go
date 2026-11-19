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
	if e.gpus != nil {
		return *e.gpus
	}
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
	e.gpus = &found
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

// Suggested tells whether an app should start selected on this machine: an
// app for a kind of graphics card (config.App.GPU) only when the machine has
// one. why explains the choice for the app's GPU apps ("" for the others).
func (e *Env) Suggested(ctx context.Context, a config.App) (ok bool, why string) {
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

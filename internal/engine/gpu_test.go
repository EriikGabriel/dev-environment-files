package engine

import (
	"context"
	"testing"
)

func TestGPUVendor(t *testing.T) {
	for name, want := range map[string]string{
		"NVIDIA GeForce RTX 5050 | NVIDIA":                           "nvidia",
		"AMD Radeon RX 7800 XT | Advanced Micro Devices, Inc.":       "amd",
		"AMD Radeon(TM) Graphics | Advanced Micro Devices, Inc.":     "amd",
		"Intel(R) UHD Graphics 770 | Intel Corporation":              "intel",
		"Microsoft Basic Display Adapter | (Standard display types)": "",
		"Parsec Virtual Display Adapter | Parsec Cloud, Inc.":        "",
	} {
		if got := GPUVendor(name); got != want {
			t.Errorf("GPUVendor(%q) = %q", name, got)
		}
	}
}

// gpuDefaults returns which of the GPU apps of the gaming profile start
// selected, in the wizard and in a plan without a selection.
func gpuDefaults(t *testing.T, answer string) (wizard, plan map[string]bool) {
	t.Helper()
	fr := &fakeRunner{answers: map[string]string{}}
	if answer != "" {
		fr.answers["AdapterCompatibility }"] = answer
	}
	env := &Env{Info: win, Root: t.TempDir(), Home: t.TempDir(), Runner: fr, StateDir: t.TempDir()}
	steps, err := Wizard(context.Background(), realConfig(t), "gaming-windows", env)
	must(t, err)
	wizard = map[string]bool{}
	for _, st := range steps {
		for _, a := range st.Apps {
			if a.ID == "nvidia-app" || a.ID == "amd-software" {
				wizard[a.ID] = a.Default
			}
		}
	}
	p, err := BuildPlan(realConfig(t), "gaming-windows", env, Options{NoFiles: true})
	must(t, err)
	plan = map[string]bool{}
	for _, s := range p.Steps {
		if as, ok := s.(*appStep); ok && (as.res.ID == "nvidia-app" || as.res.ID == "amd-software") {
			plan[as.res.ID] = true
		}
	}
	return wizard, plan
}

func TestGPUDriverFollowsTheCard(t *testing.T) {
	w, p := gpuDefaults(t, "NVIDIA GeForce RTX 5050 | NVIDIA\r\nAMD Radeon(TM) Graphics | Advanced Micro Devices, Inc.\r\n")
	if !w["nvidia-app"] || !w["amd-software"] || !p["nvidia-app"] || !p["amd-software"] {
		t.Errorf("both cards: wizard %v plan %v", w, p)
	}
	w, p = gpuDefaults(t, "NVIDIA GeForce RTX 5050 | NVIDIA\r\n")
	if !w["nvidia-app"] || w["amd-software"] || !p["nvidia-app"] || p["amd-software"] {
		t.Errorf("NVIDIA: wizard %v plan %v", w, p)
	}
	w, p = gpuDefaults(t, "AMD Radeon RX 7800 XT | Advanced Micro Devices, Inc.\r\n")
	if w["nvidia-app"] || !w["amd-software"] || p["nvidia-app"] || !p["amd-software"] {
		t.Errorf("AMD: wizard %v plan %v", w, p)
	}
	// unknown card: both are suggested, the user picks
	w, p = gpuDefaults(t, "")
	if !w["nvidia-app"] || !w["amd-software"] || !p["nvidia-app"] || !p["amd-software"] {
		t.Errorf("unknown: wizard %v plan %v", w, p)
	}
}

func TestPeripheralsStepIsSelected(t *testing.T) {
	env := &Env{Info: win, Root: t.TempDir(), Home: t.TempDir(), Runner: &fakeRunner{}, StateDir: t.TempDir()}
	steps, err := Wizard(context.Background(), realConfig(t), "gaming-windows", env)
	must(t, err)
	for _, st := range steps {
		if st.ID != "peripherals" {
			continue
		}
		if len(st.Apps) != 4 {
			t.Fatalf("apps: %+v", st.Apps)
		}
		for _, a := range st.Apps {
			if !a.Default || !a.Available {
				t.Errorf("%s: default=%v available=%v (%s)", a.ID, a.Default, a.Available, a.Detail)
			}
		}
		return
	}
	t.Fatal("no peripherals step")
}

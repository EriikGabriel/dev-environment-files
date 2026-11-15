package engine

import (
	"context"
	"fmt"

	"github.com/EriikGabriel/dev-environment-files/internal/config"
)

// WizardApp is the state of one app on this host.
type WizardApp struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	// Default tells whether the app starts selected.
	Default   bool   `json:"default"`
	Available bool   `json:"available"`
	Installed bool   `json:"installed"`
	Detail    string `json:"detail"` // how it installs, or why it is unavailable
}

// WizardFile is a config file a step deploys.
type WizardFile struct {
	Src  string `json:"src"`  // relative to the repo root
	Dest string `json:"dest"` // with ~
	Mode string `json:"mode"` // link or copy
	App  string `json:"app"`  // deployed only with this app; "" = always
}

// WizardOption is one alternative of a choice step.
type WizardOption struct {
	ID          string       `json:"id"`
	Title       string       `json:"title"`
	Description string       `json:"description"`
	Apps        []WizardApp  `json:"apps"`
	Files       []WizardFile `json:"files"`
	Dirs        []string     `json:"dirs"`
}

// WizardStep is one stage of the setup, resolved for this host.
type WizardStep struct {
	ID          string         `json:"id"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Apps        []WizardApp    `json:"apps"`
	Files       []WizardFile   `json:"files"`
	Dirs        []string       `json:"dirs"`
	ChoiceTitle string         `json:"choice_title"`
	Options     []WizardOption `json:"options"`
	Default     string         `json:"default"`
}

// Wizard describes every step of a profile with the current state of each
// app, so a front-end can walk the user through the choices.
func Wizard(ctx context.Context, cfg *config.Config, profileID string, env *Env) ([]WizardStep, error) {
	prof, ok := cfg.Profiles[profileID]
	if !ok {
		return nil, fmt.Errorf("unknown profile %q", profileID)
	}
	app := func(id string, def bool) WizardApp {
		a := cfg.Apps[id]
		w := WizardApp{ID: id, Description: a.Description, Default: def}
		res, err := Resolve(id, a, env.Info)
		if err != nil {
			w.Default, w.Detail = false, err.Error()
			return w
		}
		st := &appStep{env: env, res: res}
		w.Available = true
		w.Installed = st.installed(ctx)
		w.Detail = st.describeInstall()
		return w
	}
	dests := func(files []config.File) []WizardFile {
		out := []WizardFile{}
		for _, f := range files {
			if appliesTo(f.Only, env.Info) {
				mode := f.Mode
				if mode == "" {
					mode = "link"
				}
				out = append(out, WizardFile{Src: f.Src, Dest: f.Dest, Mode: mode, App: f.App})
			}
		}
		return out
	}

	steps := []WizardStep{}
	for _, st := range prof.Steps {
		w := WizardStep{
			ID: st.ID, Title: st.Title, Description: st.Description,
			Apps: []WizardApp{}, Files: dests(st.Files), Dirs: append([]string{}, st.Dirs...),
			ChoiceTitle: st.ChoiceTitle, Options: []WizardOption{}, Default: st.Default,
		}
		for _, id := range st.Apps {
			w.Apps = append(w.Apps, app(id, true))
		}
		for _, id := range st.Optional {
			w.Apps = append(w.Apps, app(id, false))
		}
		for _, o := range st.Options {
			wo := WizardOption{ID: o.ID, Title: o.Title, Description: o.Description, Apps: []WizardApp{}, Files: dests(o.Files), Dirs: append([]string{}, o.Dirs...)}
			if wo.Title == "" {
				wo.Title = o.ID
			}
			for _, id := range o.Apps {
				wo.Apps = append(wo.Apps, app(id, true))
			}
			if o.VSCodeExtensions != "" {
				wo.Files = append(wo.Files, WizardFile{Src: o.VSCodeExtensions, Dest: "extensões do VS Code", Mode: "install"})
			}
			w.Options = append(w.Options, wo)
		}
		steps = append(steps, w)
	}
	return steps, nil
}

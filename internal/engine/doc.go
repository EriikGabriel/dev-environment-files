// Package engine turns a profile plus the detected platform into an ordered
// list of idempotent steps, and runs them.
//
// The files are grouped by prefix:
//
//   - no prefix: the core shared by every platform (plan, resolve, steps,
//     the wizard, add/show for dotfiles)
//   - linux_*: Linux-only steps (GNOME/Noctalia desktop and dock, apt
//     repositories)
//   - win_*: Windows-only features (group policies, taskbar layout, icons,
//     Ninite, Steam, Playnite, Windhawk) and their PowerShell helpers
//
// The prefixes are names only. Suffixes like _windows.go or _linux.go are
// avoided on purpose: Go would read them as build constraints and drop the
// file when building for the other system.
package engine

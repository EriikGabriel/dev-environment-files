// Builds the Go `envsetup` binary into src-tauri/binaries/ under the
// target-triple name Tauri expects for externalBin sidecars.
import { execFileSync } from "node:child_process";
import { mkdirSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const repo = resolve(here, "..", "..");
const host = execFileSync("rustc", ["-vV"], { encoding: "utf8" })
  .split("\n")
  .find((l) => l.startsWith("host:"))
  ?.split(":")[1]
  .trim();
if (!host) throw new Error("could not read the rust host triple from `rustc -vV`");

const triple = process.env.TAURI_ENV_TARGET_TRIPLE || host;
const goos = triple.includes("windows") ? "windows" : triple.includes("darwin") ? "darwin" : "linux";
const goarch = triple.startsWith("aarch64") ? "arm64" : "amd64";
const ext = goos === "windows" ? ".exe" : "";

const outDir = join(here, "..", "src-tauri", "binaries");
mkdirSync(outDir, { recursive: true });
const out = join(outDir, `envsetup-${triple}${ext}`);

execFileSync("go", ["build", "-trimpath", "-o", out, "./cmd/envsetup"], {
  cwd: repo,
  stdio: "inherit",
  env: { ...process.env, CGO_ENABLED: "0", GOOS: goos, GOARCH: goarch },
});
console.log(`sidecar: ${out}`);

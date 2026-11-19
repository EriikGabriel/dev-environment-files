/**
 * The moving glow behind the app. Three blurred blobs in the accent color of
 * the environment drift slowly; it sits under everything and takes no clicks.
 */
export function Aurora() {
  return (
    <div className="aurora" aria-hidden="true">
      <i />
      <i />
      <i />
      <div className="grain" />
    </div>
  );
}

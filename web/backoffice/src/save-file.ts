// The click only starts the save; a URL revoked in the same tick can cut it
// short in some browsers. A moment later is soon enough.
const revokeAfterMs = 1_000;

/** Hands text to the browser as a file to save. */
export function saveFile(filename: string, text: string, type: string): void {
  const url = URL.createObjectURL(new Blob([text], { type }));
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.append(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(url), revokeAfterMs);
}

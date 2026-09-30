// Getting an item photo ready to upload (ADR-110). The browser shrinks it:
// canvas applies a phone photo's EXIF rotation, which the server's Go
// decoder would ignore, and a 480px JPEG is small enough for every till to
// fetch over the venue's wifi. The server checks it again.

/** The longest side of a stored photo, in pixels. */
export const photoSide = 480;
/** The smallest side the server accepts. */
export const minPhotoSide = 64;
const quality = 0.82;

/** A size scaled down so its longer side is at most max, keeping its shape. Never scaled up. */
export function fitWithin(width: number, height: number, max: number): { width: number; height: number } {
  const scale = Math.min(1, max / Math.max(width, height));
  return { width: Math.max(1, Math.round(width * scale)), height: Math.max(1, Math.round(height * scale)) };
}

/** Why a photo of this size cannot be used, or null. */
export function sizeProblem(width: number, height: number): string | null {
  if (width < minPhotoSide || height < minPhotoSide) return `That photo is too small. Choose one at least ${minPhotoSide} pixels on each side.`;
  return null;
}

/** A chosen file as the JPEG to upload. Throws an Error whose message the form can show. */
export async function shrinkPhoto(file: File): Promise<Blob> {
  if (!file.type.startsWith("image/")) throw new Error("Choose a photo: a JPEG or PNG file.");
  let bitmap: ImageBitmap;
  try {
    bitmap = await createImageBitmap(file, { imageOrientation: "from-image" });
  } catch {
    throw new Error("That photo cannot be read. Try another, or save it as a JPEG first.");
  }
  try {
    const size = fitWithin(bitmap.width, bitmap.height, photoSide);
    const problem = sizeProblem(size.width, size.height);
    if (problem) throw new Error(problem);
    const canvas = document.createElement("canvas");
    canvas.width = size.width;
    canvas.height = size.height;
    const ctx = canvas.getContext("2d");
    if (!ctx) throw new Error("This browser cannot prepare photos.");
    // A transparent PNG would turn black as a JPEG; the tiles are light enough for white.
    ctx.fillStyle = "white";
    ctx.fillRect(0, 0, size.width, size.height);
    ctx.drawImage(bitmap, 0, 0, size.width, size.height);
    return await new Promise<Blob>((resolve, reject) =>
      canvas.toBlob((b) => (b ? resolve(b) : reject(new Error("This browser cannot prepare photos."))), "image/jpeg", quality),
    );
  } finally {
    bitmap.close();
  }
}

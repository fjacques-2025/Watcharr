// Photos are resized in the browser before they're sent, because the server's
// image store refuses anything over 10 megapixels (a 12 MP phone photo would
// be rejected), can't read HEIC, and ignores EXIF orientation (a portrait
// photo would come out lying down). Re-encoding also drops the GPS location.

/** Long edge of the uploaded image, in pixels. */
export const MAX_EDGE = 2048;
const QUALITY = 0.85;

export const UNREADABLE_MESSAGE =
	"This photo couldn't be read by your browser — try exporting it as JPEG.";

/** The browser couldn't decode the file (e.g. HEIC outside Safari). */
export class UnreadableImageError extends Error {
	constructor() {
		super(UNREADABLE_MESSAGE);
		this.name = "UnreadableImageError";
	}
}

type Decoded = {
	source: CanvasImageSource;
	width: number;
	height: number;
	release: () => void;
};

async function decodeWithBitmap(file: Blob): Promise<Decoded> {
	const bmp = await createImageBitmap(file, { imageOrientation: "from-image" });
	return {
		source: bmp,
		width: bmp.width,
		height: bmp.height,
		release: () => bmp.close(),
	};
}

// Fallback for browsers whose createImageBitmap can't take the file (or the
// option) while their <img> decoder can; <img> applies EXIF orientation.
async function decodeWithImg(file: Blob): Promise<Decoded> {
	const url = URL.createObjectURL(file);
	try {
		const img = new Image();
		img.src = url;
		await img.decode();
		if (!img.naturalWidth || !img.naturalHeight) throw new Error("empty");
		return {
			source: img,
			width: img.naturalWidth,
			height: img.naturalHeight,
			release: () => URL.revokeObjectURL(url),
		};
	} catch (err) {
		URL.revokeObjectURL(url);
		throw err;
	}
}

async function decode(file: Blob): Promise<Decoded> {
	try {
		return await decodeWithBitmap(file);
	} catch (err) {
		console.warn("resizeImage: createImageBitmap failed, trying <img>", err);
	}
	try {
		return await decodeWithImg(file);
	} catch (err) {
		console.warn("resizeImage: <img> decode failed", err);
		throw new UnreadableImageError();
	}
}

/**
 * Decodes an image file (orientation applied), scales it so its long edge is
 * at most `MAX_EDGE` (never upscaled) and re-encodes it as JPEG.
 * Throws `UnreadableImageError` when the browser can't read the file.
 */
export async function resizeImage(file: Blob): Promise<Blob> {
	const d = await decode(file);
	try {
		const scale = Math.min(1, MAX_EDGE / Math.max(d.width, d.height));
		const w = Math.max(1, Math.round(d.width * scale));
		const h = Math.max(1, Math.round(d.height * scale));
		const canvas = document.createElement("canvas");
		canvas.width = w;
		canvas.height = h;
		const ctx = canvas.getContext("2d");
		if (!ctx) throw new Error("no 2D canvas available");
		// JPEG has no transparency: a transparent PNG would turn black.
		ctx.fillStyle = "#fff";
		ctx.fillRect(0, 0, w, h);
		ctx.imageSmoothingEnabled = true;
		ctx.imageSmoothingQuality = "high";
		ctx.drawImage(d.source, 0, 0, w, h);
		const blob = await new Promise<Blob | null>((res) =>
			canvas.toBlob(res, "image/jpeg", QUALITY),
		);
		if (!blob) throw new Error("could not encode the photo");
		return blob;
	} finally {
		d.release();
	}
}

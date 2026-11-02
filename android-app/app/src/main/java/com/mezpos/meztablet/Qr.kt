package com.mezpos.meztablet

import com.google.zxing.BinaryBitmap
import com.google.zxing.DecodeHintType
import com.google.zxing.PlanarYUVLuminanceSource
import com.google.zxing.ReaderException
import com.google.zxing.common.HybridBinarizer
import com.google.zxing.qrcode.QRCodeReader

private val HINTS = mapOf(DecodeHintType.TRY_HARDER to true)

/**
 * The text of a QR code in a camera frame's brightness plane, or null if it
 * holds none that reads. [luminance] is [height] rows of [rowStride] bytes,
 * the first [width] of each the picture's (Android pads rows); the last row
 * may stop at [width]. Any way up: a QR code reads turned.
 */
fun qrText(luminance: ByteArray, rowStride: Int, width: Int, height: Int): String? {
    if (width <= 0 || height <= 0 || rowStride < width) return null
    val rows = if (luminance.size >= rowStride * height) luminance else luminance.copyOf(rowStride * height)
    val source = PlanarYUVLuminanceSource(rows, rowStride, height, 0, 0, width, height, false)
    return try {
        QRCodeReader().decode(BinaryBitmap(HybridBinarizer(source)), HINTS).text
    } catch (e: ReaderException) {
        null
    }
}

/**
 * The brightness of each of [pixels] (Android's ARGB, as a Bitmap gives
 * them), as [qrText] reads them: the eye's weights for red, green and blue,
 * in whole numbers.
 */
fun luminance(pixels: IntArray): ByteArray = ByteArray(pixels.size) { i ->
    val argb = pixels[i]
    val red = (argb shr RED) and BYTE
    val green = (argb shr GREEN) and BYTE
    val blue = argb and BYTE
    ((red * RED_WEIGHT + green * GREEN_WEIGHT + blue * BLUE_WEIGHT) shr WEIGHT_BITS).toByte()
}

private const val RED = 16
private const val GREEN = 8
private const val BYTE = 0xFF
private const val RED_WEIGHT = 77
private const val GREEN_WEIGHT = 150
private const val BLUE_WEIGHT = 29
private const val WEIGHT_BITS = 8

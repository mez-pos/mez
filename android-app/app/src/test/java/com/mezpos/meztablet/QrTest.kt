package com.mezpos.meztablet

import com.google.zxing.BarcodeFormat
import com.google.zxing.common.BitMatrix
import com.google.zxing.qrcode.QRCodeWriter
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class QrTest {

    private val link = "http://192.168.1.20:7777/kitchen#code=MW4C-HHWY"

    /** [matrix] as a camera's brightness plane: dark 20, light 230, rows padded to [rowStride]. */
    private fun frame(matrix: BitMatrix, rowStride: Int, lastRowShort: Boolean = false): ByteArray {
        val size = rowStride * matrix.height - if (lastRowShort) rowStride - matrix.width else 0
        val bytes = ByteArray(size) { PADDING }
        for (y in 0 until matrix.height) {
            for (x in 0 until matrix.width) bytes[y * rowStride + x] = if (matrix[x, y]) DARK else LIGHT
        }
        return bytes
    }

    private fun drawn(text: String, side: Int = 400): BitMatrix = QRCodeWriter().encode(text, BarcodeFormat.QR_CODE, side, side)

    @Test
    fun `a pairing link read from a frame`() {
        val matrix = drawn(link)
        assertEquals(link, qrText(frame(matrix, rowStride = matrix.width), matrix.width, matrix.width, matrix.height))
    }

    @Test
    fun `rows padded past the picture, as Android pads them`() {
        val matrix = drawn(link)
        val stride = matrix.width + 64
        assertEquals(link, qrText(frame(matrix, stride), stride, matrix.width, matrix.height))
        assertEquals("the last row cut short", link,
            qrText(frame(matrix, stride, lastRowShort = true), stride, matrix.width, matrix.height))
    }

    @Test
    fun `a code held on its side`() {
        val matrix = drawn(link)
        val turned = BitMatrix(matrix.height, matrix.width)
        for (y in 0 until matrix.height) for (x in 0 until matrix.width) if (matrix[x, y]) turned.set(matrix.height - 1 - y, x)
        assertEquals(link, qrText(frame(turned, turned.width), turned.width, turned.width, turned.height))
    }

    @Test
    fun `a code read from what the screen shows, in colour`() {
        val matrix = drawn(link, side = 300)
        // The page's scrim and a warm light, as the preview copy has them.
        val pixels = IntArray(matrix.width * matrix.height) { i ->
            if (matrix[i % matrix.width, i / matrix.width]) 0xFF2A2420.toInt() else 0xFFF4EBDD.toInt()
        }
        assertEquals(link, qrText(luminance(pixels), matrix.width, matrix.width, matrix.height))
    }

    @Test
    fun `brightness by the eye's weights`() {
        val shades = luminance(intArrayOf(0xFF000000.toInt(), 0xFFFFFFFF.toInt(), 0xFFFF0000.toInt(), 0xFF00FF00.toInt(), 0xFF0000FF.toInt()))
        assertEquals(listOf(0, 255, 76, 149, 28), shades.map { it.toInt() and 0xFF })
    }

    @Test
    fun `nothing to read`() {
        val blank = BitMatrix(320, 240)
        assertNull("a blank frame", qrText(frame(blank, 320), 320, 320, 240))
        assertNull("no picture", qrText(ByteArray(0), 0, 0, 0))
        assertNull("a stride shorter than a row", qrText(ByteArray(100), 5, 10, 10))
    }

    private companion object {
        const val DARK: Byte = 20
        const val LIGHT: Byte = -26 // 230, unsigned
        const val PADDING: Byte = 0x55
    }
}

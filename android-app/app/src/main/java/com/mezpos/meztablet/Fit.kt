package com.mezpos.meztablet

/**
 * How the camera's picture lies in the TextureView ([Scanner]): as the
 * texture draws it ([drawnW] by [drawnH]), the quarter turns still to make
 * ([turn], clockwise degrees), and its shape once turned ([shownW] by
 * [shownH]), which is scaled to fill the screen without squashing it.
 */
data class Fit(val drawnW: Int, val drawnH: Int, val turn: Int, val shownW: Int, val shownH: Int)

private const val TURN = 360
private const val HALF = 180

/**
 * The fit of a [width] by [height] camera picture that wants [rotation]
 * degrees to stand upright on the screen (CameraX's `rotationDegrees`), of
 * which the texture has made [applied] already: the camera turns its picture
 * to the tablet's natural way up itself when it draws into a SurfaceTexture
 * (`hasCameraTransform`, the sensor's rotation). Turning by the whole of
 * [rotation] again turned the picture a quarter too far and stretched it.
 */
fun fit(width: Int, height: Int, rotation: Int, applied: Int): Fit {
    val drawnTurned = applied % HALF != 0
    val drawnW = if (drawnTurned) height else width
    val drawnH = if (drawnTurned) width else height
    val turn = ((rotation - applied) % TURN + TURN) % TURN
    val turned = turn % HALF != 0
    return Fit(
        drawnW = drawnW,
        drawnH = drawnH,
        turn = turn,
        shownW = if (turned) drawnH else drawnW,
        shownH = if (turned) drawnW else drawnH,
    )
}

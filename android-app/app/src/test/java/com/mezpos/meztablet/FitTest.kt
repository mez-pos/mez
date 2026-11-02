package com.mezpos.meztablet

import org.junit.Assert.assertEquals
import org.junit.Test

class FitTest {

    @Test
    fun `the camera's picture, laid upright on the screen`() {
        val cases = listOf(
            // A tablet whose sensor sits a quarter turn from its natural way
            // up, held that way: the texture has turned it, nothing is left.
            Triple("the sensor's turn made by the texture", fit(640, 480, rotation = 90, applied = 90), Fit(480, 640, 0, 480, 640)),
            // The same, turned by hand as well: what the first build did.
            Triple("no turn made by the texture", fit(640, 480, rotation = 90, applied = 0), Fit(640, 480, 90, 480, 640)),
            Triple("a sensor the right way up", fit(1280, 720, rotation = 0, applied = 0), Fit(1280, 720, 0, 1280, 720)),
            Triple("the tablet turned a quarter from its sensor", fit(640, 480, rotation = 0, applied = 90), Fit(480, 640, 270, 640, 480)),
            Triple("upside down", fit(640, 480, rotation = 270, applied = 90), Fit(480, 640, 180, 480, 640)),
            Triple("a front camera's three quarters", fit(640, 480, rotation = 270, applied = 270), Fit(480, 640, 0, 480, 640)),
            Triple("a landscape sensor, the tablet upside down", fit(1280, 720, rotation = 180, applied = 0), Fit(1280, 720, 180, 1280, 720)),
            Triple("a front camera, the tablet turned", fit(640, 480, rotation = 0, applied = 270), Fit(480, 640, 90, 640, 480)),
            Triple("a rotation past a whole turn", fit(640, 480, rotation = 450, applied = 0), Fit(640, 480, 90, 480, 640)),
        )
        for ((name, got, want) in cases) assertEquals(name, want, got)
    }
}

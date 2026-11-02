package com.mezpos.meztablet

import org.junit.Assert.assertEquals
import org.junit.Test

class RevivalTest {

    @Test
    fun `the screen comes back once, and stops for a person when it keeps going`() {
        val now = 10 * 60_000L
        val cases = listOf(
            Triple("the first time", null, Revival.RELOAD),
            Triple("an hour after the last", now - 60 * 60_000L, Revival.RELOAD),
            Triple("a minute to the millisecond after the last", now - GONE_AGAIN_MS, Revival.RELOAD),
            Triple("just inside the minute", now - GONE_AGAIN_MS + 1, Revival.STOP),
            Triple("seconds after the last", now - 2_000L, Revival.STOP),
            Triple("at the same moment", now, Revival.STOP),
        )
        for ((name, lastGone, want) in cases) assertEquals(name, want, revival(now, lastGone))
    }
}

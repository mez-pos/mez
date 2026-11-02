package com.mezpos.meztablet

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class WayOutTest {

    private fun hold() = Hold(holdMs = 3_000, slop = 20f)

    @Test
    fun `two fingers held still for three seconds make the hold`() {
        val hold = hold()
        assertNull("one finger is the till's", hold.touch(1, floatArrayOf(100f, 100f), at = 0))
        assertEquals("the second finger starts it", 3_100L, hold.touch(2, TWO, at = 100))
        assertEquals("still, it is still due then", 3_100L, hold.touch(2, TWO.nudged(5f), at = 1_000))
        assertFalse(hold.made(now = 3_099))
        assertTrue(hold.made(now = 3_100))
    }

    @Test
    fun `and nothing else does`() {
        val moved = hold()
        moved.touch(2, TWO, at = 0)
        assertNull("a finger moved past the slop: a pinch or a scroll", moved.touch(2, TWO.nudged(40f), at = 500))
        assertFalse(moved.made(now = 5_000))
        assertNull("and it stays spoiled until the fingers lift", moved.touch(2, TWO, at = 600))

        val lifted = hold()
        lifted.touch(2, TWO, at = 0)
        assertNull(lifted.touch(1, floatArrayOf(100f, 100f), at = 1_000))
        assertFalse("lifted before the time", lifted.made(now = 3_000))

        val three = hold()
        three.touch(2, TWO, at = 0)
        assertNull(three.touch(3, TWO + floatArrayOf(500f, 500f), at = 1_000))
        assertFalse("a third finger", three.made(now = 3_000))
    }

    @Test
    fun `lifted and held again, the hold starts again`() {
        val hold = hold()
        hold.touch(2, TWO, at = 0)
        hold.touch(2, TWO.nudged(40f), at = 100)
        hold.touch(0, floatArrayOf(), at = 200)
        assertEquals(3_300L, hold.touch(2, TWO, at = 300))
        assertTrue(hold.made(now = 3_300))
        hold.reset()
        assertFalse("made once, it is not made again", hold.made(now = 9_000))
    }

    @Test
    fun `what the PC said of the PIN`() {
        assertEquals(Opened.OPEN, openedOf(200, "{\"opened_by\":{\"id\":\"stf_1\",\"name\":\"Priya\"}}", staff = "stf_1"))
        assertEquals("a yes for someone else is no yes", Opened.NO_ANSWER, openedOf(200, "{\"opened_by\":{\"id\":\"stf_2\"}}", staff = "stf_1"))
        assertEquals("a 200 that says nothing is no yes", Opened.NO_ANSWER, openedOf(200, "ok", staff = "stf_1"))
        assertEquals(Opened.WRONG_PIN, openedOf(401, "{\"error\":{\"code\":\"invalid_pin\"}}", staff = "stf_1"))
        assertEquals(Opened.CANNOT, openedOf(403, "{\"error\":{\"code\":\"forbidden\"}}", staff = "stf_1"))
        assertEquals(Opened.LOCKED, openedOf(429, "{\"error\":{\"code\":\"locked_out\"}}", staff = "stf_1"))
        assertEquals("the tablet no longer paired", Opened.NOT_PAIRED, openedOf(401, "{\"error\":{\"code\":\"unauthenticated\"}}", staff = "stf_1"))
        assertEquals(Opened.NOT_PAIRED, openedOf(403, "{\"error\":{\"code\":\"device_revoked\"}}", staff = "stf_1"))
        assertEquals("anything else", Opened.NO_ANSWER, openedOf(500, "{}", staff = "stf_1"))
        assertEquals("a PC from before ADR-419", Opened.NO_ANSWER, openedOf(404, "{\"error\":{\"code\":\"not_found\"}}", staff = "stf_1"))
    }

    @Test
    fun `who opened it, read plainly`() {
        assertEquals("Priya", openedByOf("{\"opened_by\":{\"id\":\"stf_1\",\"name\":\"Priya\",\"role\":\"owner\"}}"))
        assertNull("a name with an escape is not read", openedByOf("{\"opened_by\":{\"name\":\"Pri\\\"ya\"}}"))
        assertNull(openedByOf("{}"))
    }

    @Test
    fun `who may open it, as the PC answered`() {
        val who = "{\"staff\":[{\"id\":\"stf_PRIYA001\",\"name\":\"Priya\",\"role\":\"owner\"}]}"
        assertEquals(who, whoOf(200, who))
        assertNull("not a 200", whoOf(404, who))
        assertNull("not the list", whoOf(200, "<html></html>"))
        assertNull("too long to put in a page's address", whoOf(200, "{\"staff\":[" + "x".repeat(20_000) + "]}"))
    }

    @Test
    fun `what is sent for the PIN`() {
        assertEquals("{\"authorised_by\":\"stf_PRIYA001\",\"pin\":\"2468\"}", wayOutBody("stf_PRIYA001", "2468"))
        assertNull("not a staff id", wayOutBody("stf_\"x", "2468"))
        assertNull("not a PIN", wayOutBody("stf_PRIYA001", "24\"68"))
        assertNull("no PIN", wayOutBody("stf_PRIYA001", ""))
        assertEquals("http://192.168.1.45:7777/api/v1/devices/me/way-out", wayOutUrl("http://192.168.1.45:7777/kitchen"))
        assertNull(wayOutUrl(AppPages.page(AppPages.CHECKING)))
    }

    @Test
    fun `the way out's pages carry what they show`() {
        val page = AppPages.wayOut(who = "{\"staff\":[]}", why = "wrong-pin")
        assertTrue(page.startsWith(AppPages.ASSETS + "start.html?"))
        assertTrue(page.endsWith("#way-out"))
        assertTrue(page.contains("why=wrong-pin"))
        assertEquals("{\"staff\":[]}", queryOf(page.substringBefore('#'), "who"))
        assertEquals("Priya & Co", queryOf(AppPages.wayOutOpen("Priya & Co").substringBefore('#'), "by"))
        assertEquals("2468", queryOf("mez-app:way-out-open?staff=stf_1&pin=2468", "pin"))
        assertNull(queryOf("mez-app:way-out-open?staff=stf_1", "pin"))
    }

    private companion object {
        val TWO = floatArrayOf(100f, 100f, 300f, 120f)

        fun FloatArray.nudged(by: Float) = FloatArray(size) { this[it] + by }
    }
}

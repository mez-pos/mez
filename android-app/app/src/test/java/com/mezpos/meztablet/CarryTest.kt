package com.mezpos.meztablet

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class CarryTest {

    @Test
    fun `the app's empty page is at the root of the PC's address`() {
        assertEquals("http://192.168.1.20:7777/mez-app-carry", carryPage("http://192.168.1.20:7777/kitchen"))
        assertEquals("http://mez.local/mez-app-carry", carryPage("http://mez.local/"))
        assertNull("no address", carryPage(""))
        assertNull("one of the app's own pages", carryPage(AppPages.page(AppPages.UNREACHABLE)))
    }

    @Test
    fun `only the empty page is the empty page`() {
        assertTrue(isCarryPage("http://10.0.2.2:7777/mez-app-carry"))
        assertFalse(isCarryPage("http://10.0.2.2:7777/"))
        assertFalse(isCarryPage("http://10.0.2.2:7777/kitchen"))
        assertFalse(isCarryPage("http://10.0.2.2:7777/mez-app-carry/more"))
        assertFalse(isCarryPage(AppPages.page(AppPages.CHECKING)))
    }

    @Test
    fun `the page a carry serves is matched as an address`() {
        val page = "http://192.168.1.20:80/mez-app-carry"
        assertTrue("WebView leaves :80 off", atCarryPage("http://192.168.1.20/mez-app-carry", page))
        assertFalse("another PC's", atCarryPage("http://192.168.1.45/mez-app-carry", page))
        assertFalse("the PC's till", atCarryPage("http://192.168.1.20/", page))
        assertFalse("no carry under way", atCarryPage("http://192.168.1.20/mez-app-carry", null))
    }

    @Test
    fun `the pairing goes to the same venue's mez at another address`() {
        assertTrue(carries(OLD, VENUE, to = NEW, toVenue = VENUE))
        assertTrue("the kitchen display's", carries("$OLD_PC/kitchen", VENUE, to = "$NEW_PC/kitchen", toVenue = VENUE))
    }

    @Test
    fun `and nowhere else`() {
        assertFalse("another venue's mez", carries(OLD, VENUE, to = NEW, toVenue = "ven_DEM0"))
        assertFalse("a mez that does not say", carries(OLD, VENUE, to = NEW, toVenue = null))
        assertFalse("a venue the app never heard", carries(OLD, null, to = NEW, toVenue = VENUE))
        assertFalse("nothing held", carries("", VENUE, to = NEW, toVenue = VENUE))
        // The pairing is already there: one address, one storage.
        assertFalse("the same address", carries(OLD, VENUE, to = OLD, toVenue = VENUE))
        assertFalse("the same address, written longer", carries("http://192.168.1.20:80/", VENUE, to = "http://192.168.1.20/", toVenue = VENUE))
        assertFalse("the same PC's other screen", carries(OLD, VENUE, to = "$OLD_PC/kitchen", toVenue = VENUE))
    }

    @Test
    fun `what the old address held is written at the new one, as it was read`() {
        // As evaluateJavascript hands back a string: JSON, quotes and all.
        val read = "\"{\\\"mez.terminal.device\\\":\\\"x\\\"}\""
        val script = requireNotNull(writeKeys(read))
        assertTrue(script.endsWith(")($read)"))
        assertTrue(script.startsWith("(function (text) {"))
    }

    @Test
    fun `nothing is written that the read did not answer as a string`() {
        assertNull("no pairing at the old address", writeKeys("null"))
        assertNull("not a string", writeKeys("{\"mez.terminal.device\":\"x\"}"))
        assertNull("an empty string", writeKeys("\"\""))
        assertNull("unterminated", writeKeys("\"abc"))
        assertNull("an escaped end", writeKeys("\"abc\\\""))
        assertNull("more after the string", writeKeys("\"a\"+alert(1)+\"b\""))
        assertNull("a line in it", writeKeys("\"a\nb\""))
    }

    @Test
    fun `a JSON string is read whole`() {
        assertTrue(isJsonString("\"a\""))
        assertTrue("an escaped quote inside", isJsonString("\"a\\\"b\""))
        assertTrue("an escaped backslash at the end", isJsonString("\"a\\\\\""))
        assertFalse(isJsonString("\"a\"\"b\""))
        assertFalse(isJsonString("a"))
        assertFalse(isJsonString("\""))
    }

    private companion object {
        const val VENUE = "ven_7K3M9Q"
        const val OLD_PC = "http://192.168.1.20:7777"
        const val NEW_PC = "http://192.168.1.45:7777"
        const val OLD = "$OLD_PC/"
        const val NEW = "$NEW_PC/"
    }
}

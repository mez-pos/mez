package com.mezpos.meztablet

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class LinkTest {

    private fun link(pc: String, app: App, code: String? = null) =
        Pairing.Link(pc = pc, open = if (code == null) pc else "$pc#code=$code", app = app)

    private fun refused(why: Why) = Pairing.Refused(why)

    @Test
    fun `the Devices page's links, scanned`() {
        val cases = listOf(
            Triple("the till's pairing QR", "http://192.168.1.20:7777/#code=MW4C-HHWY",
                link("http://192.168.1.20:7777/", App.TILL, "MW4C-HHWY")),
            Triple("the kitchen display's pairing QR", "http://192.168.1.20:7777/kitchen#code=MW4C-HHWY",
                link("http://192.168.1.20:7777/kitchen", App.KITCHEN, "MW4C-HHWY")),
            Triple("an address QR, by name, no code", "http://mez.local:7777",
                link("http://mez.local:7777/", App.TILL)),
            Triple("the kitchen by name", "http://mez.local:7777/kitchen",
                link("http://mez.local:7777/kitchen", App.KITCHEN)),
            Triple("https", "https://10.0.0.5:7443/kitchen#code=ABCD-2345",
                link("https://10.0.0.5:7443/kitchen", App.KITCHEN, "ABCD-2345")),
            Triple("no port", "http://192.168.0.4/", link("http://192.168.0.4/", App.TILL)),
            Triple("the emulator's PC", "http://10.0.2.2:7777/", link("http://10.0.2.2:7777/", App.TILL)),
        )
        for ((name, text, want) in cases) assertEquals(name, want, readLink(text))
    }

    @Test
    fun `an address typed as the PC shows it`() {
        val cases = listOf(
            Triple("host and port", "192.168.1.20:7777", link("http://192.168.1.20:7777/", App.TILL)),
            Triple("the kitchen", "192.168.1.20:7777/kitchen", link("http://192.168.1.20:7777/kitchen", App.KITCHEN)),
            Triple("spaces around it", "  192.168.1.20:7777  ", link("http://192.168.1.20:7777/", App.TILL)),
            Triple("by name, in capitals", "MEZ.LOCAL:7777/Kitchen/", link("http://mez.local:7777/kitchen", App.KITCHEN)),
            Triple("the scheme in capitals", "HTTP://172.16.0.9:7777", link("http://172.16.0.9:7777/", App.TILL)),
            Triple("a code typed in small letters, no hyphen", "192.168.1.20:7777/#code=mw4chhwy",
                link("http://192.168.1.20:7777/", App.TILL, "MW4C-HHWY")),
        )
        for ((name, text, want) in cases) assertEquals(name, want, readLink(text))
    }

    @Test
    fun `not an address`() {
        val texts = listOf(
            "", "   ", "hello there", "ftp://192.168.1.20/", "mez-app:ask", "javascript:alert(1)",
            "file:///sdcard/till.html", "http://", "http://user:pin@192.168.1.20:7777/",
            "http://192.168.1.20:7777/?x=1", "http://192.168.1.20:0/", "http://192.168.1.20:65536/",
            "http://192.168.1.20:7777/" + "a".repeat(LINK_MAX),
            // Not hosts as java.net.URI reads them.
            "http://192.168.1:7777/", "http://192.168.1.256/",
        )
        for (text in texts) assertEquals(text, refused(Why.NOT_AN_ADDRESS), readLink(text))
    }

    @Test
    fun `not a PC on the venue's network`() {
        val texts = listOf(
            "http://8.8.8.8:7777/", "https://mezpos.com/", "http://nofiltr.mezpos.com/",
            "http://127.0.0.1:7777/", "http://localhost:7777/", "http://172.32.0.1:7777/",
            "http://172.15.255.255:7777/", "http://192.169.1.1/", "http://11.0.0.1/",
            "http://[fd00::5]:7777/", "http://192.168.1.020:7777/",
            "http://mez.local.example.com/", "http://mez.local./",
            "http://my-pc.local:7777/",
        )
        for (text in texts) assertEquals(text, refused(Why.NOT_ON_THE_NETWORK), readLink(text))
    }

    @Test
    fun `the back office, and paths that are no app's`() {
        assertEquals(refused(Why.BACK_OFFICE), readLink("http://192.168.1.20:7777/backoffice/#code=MW4C-HHWY"))
        assertEquals(refused(Why.BACK_OFFICE), readLink("192.168.1.20:7777/backoffice"))
        for (text in listOf("http://192.168.1.20:7777/kitchen/extra", "http://192.168.1.20:7777/api/v1/health",
            "http://192.168.1.20:7777/pos", "http://192.168.1.20:7777//")) {
            assertEquals(text, refused(Why.NOT_AN_APP), readLink(text))
        }
    }

    @Test
    fun `a fragment that is not a pairing code`() {
        val texts = listOf(
            "http://192.168.1.20:7777/#code=MW4C", "http://192.168.1.20:7777/#code=MW4C-HHWYX",
            "http://192.168.1.20:7777/#code=MW4C-HHWY&x=1", "http://192.168.1.20:7777/#token=abc",
            "http://192.168.1.20:7777/#code=MW4C--HHWY", "http://192.168.1.20:7777/#code=MW4C%20HHWY",
        )
        for (text in texts) assertEquals(text, refused(Why.NOT_A_CODE), readLink(text))
    }

    @Test
    fun `the venue's networks`() {
        for (host in listOf("10.0.0.1", "10.255.255.255", "172.16.0.1", "172.31.255.254", "192.168.0.1", "mez.local")) {
            assertTrue(host, onTheNetwork(host))
        }
        for (host in listOf("9.255.255.255", "172.15.0.1", "172.32.0.1", "192.167.0.1", "1.1.1.1", "0.0.0.0", "10.0.0.01")) {
            assertFalse(host, onTheNetwork(host))
        }
    }

    @Test
    fun `more ways a link is written`() {
        assertEquals("no slash before the code", link("http://192.168.1.20:7777/", App.TILL, "MW4C-HHWY"),
            readLink("http://192.168.1.20:7777#code=MW4C-HHWY"))
        assertEquals("the scheme in capitals", link("https://192.168.1.20:7777/", App.TILL),
            readLink("HTTPS://192.168.1.20:7777/"))
        assertEquals("a bare #", link("http://192.168.1.20:7777/", App.TILL), readLink("http://192.168.1.20:7777/#"))
        assertEquals("an empty query", refused(Why.NOT_AN_ADDRESS), readLink("192.168.1.20:7777/?"))
        for (text in listOf("http://10.1/", "http://0x0a.0.0.1/", "http://10.0.0.1./", "http://10.0.0.1%2e/",
            "http://\u0661\u0660.0.0.1/", "http://192.168.1.20:/x", "http://192.168.1.20:7777/#a#b")) {
            assertTrue(text, readLink(text) is Pairing.Refused)
        }
        assertEquals("a host that is one number", refused(Why.NOT_ON_THE_NETWORK), readLink("http://167772161/"))
        assertEquals("a path that is not ASCII", refused(Why.NOT_AN_APP), readLink("http://192.168.1.20:7777/k\u00fcche"))
    }

    @Test
    fun `digits are ASCII digits`() {
        assertFalse(onTheNetwork("\u0661\u0660.0.0.1"))
        assertFalse(onTheNetwork("\uff11\uff10.0.0.1"))
    }

    @Test
    fun `every link read well is the PC's to the rest of the app`() {
        val texts = listOf(
            "http://192.168.1.20:7777/#code=MW4C-HHWY", "http://192.168.1.20:7777/kitchen#code=MW4C-HHWY",
            "mez.local:7777", "MEZ.LOCAL/Kitchen/", "https://10.0.0.5:7443/", "http://192.168.0.4:80/",
            "172.31.255.254:65535/kitchen",
        )
        for (text in texts) {
            val link = readLink(text) as? Pairing.Link ?: throw AssertionError("$text refused")
            assertTrue("$text: pc is its own PC", samePc(link.pc, link.pc))
            assertTrue("$text: the link opened is on the kept PC", samePc(link.open, link.pc))
            assertTrue("$text: the PC's pages may be opened", mayOpen(link.open, from = link.pc, pc = link.pc))
            assertTrue("$text: a health check", healthOf(link.pc)?.endsWith(HEALTH) == true)
            assertTrue("$text: a place to show", !place(link.pc).isNullOrEmpty())
            assertEquals("$text: read again, the same", link.copy(open = link.pc), readLink(link.pc))
        }
    }

    @Test
    fun `every refusal has its words on the page`() {
        val html = java.io.File("src/main/assets/start.html").readText()
        for (why in Why.entries) {
            assertTrue("start.html says nothing for ${why.id}", html.contains("data-why=\"${why.id}\""))
        }
    }
}

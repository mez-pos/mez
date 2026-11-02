package com.mezpos.meztablet

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.File

class AppPagesTest {

    private val mez = "http://192.168.1.20:7777/#code=ABCD-2345"
    private val own = "https://appassets.androidplatform.net/assets/start.html#ask"

    @Test
    fun `the PC's address for mez, the app's page for the rest`() {
        assertEquals(mez, AppPages.address(Start.MEZ, mez))
        val pages = mapOf(
            Start.ASK to "start.html#ask",
            Start.REFUSED to "start.html#refused",
            Start.SETTINGS to "start.html#settings",
            Start.PAIR to "start.html#pair",
        )
        for ((start, page) in pages) {
            assertEquals(start.name, AppPages.ASSETS + page, AppPages.address(start, mez))
        }
    }

    @Test
    fun `every page but mez has its section in start html`() {
        // Gradle runs unit tests in the module's directory.
        val html = File("src/main/assets/start.html").readText()
        val pages = Start.entries.mapNotNull { it.page } + AppPages.UNREACHABLE + AppPages.STOPPED + AppPages.CHECKING +
            AppPages.SCANNING
        for (page in pages) {
            assertTrue("start.html has no <section id=\"$page\">", html.contains("<section id=\"$page\">"))
        }
        assertNull(Start.MEZ.page)
    }

    @Test
    fun `can't reach the PC shows its host and port, and nothing else of the address`() {
        val page = AppPages.unreachable(mez)
        assertEquals(AppPages.ASSETS + "start.html?pc=192.168.1.20%3A7777#unreachable", page)
        assertFalse("the pairing code leaked into $page", page.contains("ABCD"))
        assertTrue(AppPages.isOwn(page))
        assertEquals(
            AppPages.ASSETS + "start.html?pc=mez.local#unreachable",
            AppPages.unreachable("http://mez.local/kitchen"),
        )
        // An address with no PC in it still gets the page, without a place.
        assertEquals(AppPages.ASSETS + "start.html#unreachable", AppPages.unreachable("about:blank"))
        assertEquals(AppPages.ASSETS + "start.html#stopped", AppPages.page(AppPages.STOPPED))
        assertEquals(
            AppPages.ASSETS + "start.html?pc=10.0.2.2%3A7777#checking",
            AppPages.checking("http://10.0.2.2:7777/kitchen"),
        )
    }

    @Test
    fun `every action start html links to is one the app knows`() {
        val html = File("src/main/assets/start.html").readText()
        val links = Regex("""href="(mez-app:[^"]*)"""").findAll(html).map { it.groupValues[1] }.toSet()
        assertTrue("start.html links to no action", links.isNotEmpty())
        val known = Action.entries.map { it.link }.toSet()
        assertTrue("unknown actions ${links - known}", known.containsAll(links))
    }

    @Test
    fun `the app's own pages are the asset origin's and no other`() {
        assertTrue(AppPages.isOwn(own))
        assertTrue(AppPages.isOwn(AppPages.address(Start.SETTINGS, mez)))
        val others = listOf(
            null,
            "",
            mez,
            "http://appassets.androidplatform.net/assets/start.html",
            "https://appassets.androidplatform.net.example.com/assets/start.html",
            "https://appassets.androidplatform.net/other/start.html",
            "https://example.com/?u=https://appassets.androidplatform.net/assets/",
        )
        for (url in others) {
            assertFalse("$url", AppPages.isOwn(url))
        }
    }

    @Test
    fun `an action asked by the app's own page`() {
        assertEquals(Action.ASK_LOCAL_NETWORK, Action.of("mez-app:ask", from = own))
        assertEquals(Action.OPEN_SETTINGS, Action.of("mez-app:settings", from = own))
        assertEquals(Action.RETRY, Action.of("mez-app:retry", from = AppPages.unreachable(mez)))
    }

    @Test
    fun `no action for a page from the network, or for a link the app does not know`() {
        assertNull(Action.of("mez-app:ask", from = mez))
        assertNull(Action.of("mez-app:retry", from = mez))
        assertNull(Action.of("mez-app:settings", from = null))
        assertNull(Action.of("mez-app:uninstall", from = own))
        assertNull(Action.of("mez-app:ask?x=1", from = own))
        assertNull(Action.of("https://example.com/", from = own))
    }

    @Test
    fun `the address typed on the pair page`() {
        val from = AppPages.page("pair")
        val link = "mez-app:address?typed=192.168.1.20%3A7777%2Fkitchen"
        assertEquals(Action.ADDRESS, Action.of(link, from))
        assertEquals("192.168.1.20:7777/kitchen", typedOf(link))
        assertEquals("", typedOf("mez-app:address?typed="))
        assertEquals("a b", typedOf("mez-app:address?typed=a%20b"))
        assertNull("nothing typed", typedOf("mez-app:address"))
        assertNull("not what was typed", typedOf("mez-app:address?other=1"))
        assertNull("broken escapes", typedOf("mez-app:address?typed=%zz"))
        assertNull("from the network", Action.of(link, from = mez))
        assertNull("a query on an action that takes none", Action.of("mez-app:pair-again?x=1", from))
        assertEquals(Action.PAIR_AGAIN, Action.of("mez-app:pair-again", from = AppPages.unreachable(mez)))
    }

    @Test
    fun `a refusal opens the pair page again, saying why, with what was typed`() {
        val page = AppPages.pairRefused(Why.BACK_OFFICE, "192.168.1.20:7777/backoffice")
        assertEquals(AppPages.ASSETS + "start.html?why=back-office&typed=192.168.1.20%3A7777%2Fbackoffice#pair", page)
        assertTrue(AppPages.isOwn(page))
        assertEquals(
            AppPages.ASSETS + "start.html?why=not-an-address&typed=a%26b%23c#pair",
            AppPages.pairRefused(Why.NOT_AN_ADDRESS, "a&b#c"),
        )
    }

    @Test
    fun `the camera, from the pair page`() {
        val pair = AppPages.page("pair")
        assertEquals(Action.SCAN, Action.of("mez-app:scan", from = pair))
        assertEquals(Action.CANCEL_SCAN, Action.of("mez-app:cancel-scan", from = AppPages.page(AppPages.SCANNING)))
        assertNull("from the network", Action.of("mez-app:scan", from = mez))
        assertEquals(
            AppPages.ASSETS + "start.html?camera=refused#pair",
            AppPages.pairWithoutCamera(AppPages.CAMERA_REFUSED),
        )
        val html = File("src/main/assets/start.html").readText()
        for (why in listOf(AppPages.CAMERA_REFUSED, AppPages.CAMERA_NONE)) {
            assertTrue("start.html says nothing for camera=$why", html.contains("data-camera=\"$why\""))
        }
    }

    @Test
    fun `every action is under the scheme the client watches`() {
        for (action in Action.entries) {
            assertTrue(action.name, action.link.startsWith(Action.SCHEME))
        }
    }
}

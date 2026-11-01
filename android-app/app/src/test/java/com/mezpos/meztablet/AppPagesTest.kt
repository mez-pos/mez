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
            Start.NO_ADDRESS to "start.html#no-address",
        )
        for ((start, page) in pages) {
            assertEquals(start.name, AppPages.ASSETS + page, AppPages.address(start, mez))
        }
    }

    @Test
    fun `every page but mez has its section in start html`() {
        // Gradle runs unit tests in the module's directory.
        val html = File("src/main/assets/start.html").readText()
        for (start in Start.entries) {
            val page = start.page ?: continue
            assertTrue("start.html has no <section id=\"$page\">", html.contains("<section id=\"$page\">"))
        }
        assertNull(Start.MEZ.page)
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
    }

    @Test
    fun `no action for a page from the network, or for a link the app does not know`() {
        assertNull(Action.of("mez-app:ask", from = mez))
        assertNull(Action.of("mez-app:settings", from = null))
        assertNull(Action.of("mez-app:uninstall", from = own))
        assertNull(Action.of("mez-app:ask?x=1", from = own))
        assertNull(Action.of("https://example.com/", from = own))
    }

    @Test
    fun `every action is under the scheme the client watches`() {
        for (action in Action.entries) {
            assertTrue(action.name, action.link.startsWith(Action.SCHEME))
        }
    }
}

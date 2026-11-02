package com.mezpos.meztablet

import android.webkit.WebViewClient
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class ReachTest {

    @Test
    fun `the errors that mean the PC did not answer`() {
        val noAnswer = mapOf(
            "no such address" to WebViewClient.ERROR_HOST_LOOKUP,
            "refused, or no route" to WebViewClient.ERROR_CONNECT,
            "no answer in time" to WebViewClient.ERROR_TIMEOUT,
            "the connection dropped" to WebViewClient.ERROR_IO,
        )
        for ((name, error) in noAnswer) assertTrue(name, noAnswer(error))

        val notMended = mapOf(
            "a bad address" to WebViewClient.ERROR_BAD_URL,
            "a scheme WebView does not open" to WebViewClient.ERROR_UNSUPPORTED_SCHEME,
            "a redirect loop" to WebViewClient.ERROR_REDIRECT_LOOP,
            "Safe Browsing" to WebViewClient.ERROR_UNSAFE_RESOURCE,
            "a file" to WebViewClient.ERROR_FILE_NOT_FOUND,
            "anything else" to WebViewClient.ERROR_UNKNOWN,
        )
        for ((name, error) in notMended) assertFalse(name, noAnswer(error))
    }

    @Test
    fun `a retry keeps the pairing code the app opened the page with`() {
        val asked = "http://192.168.1.20:7777/kitchen#code=ABCD-2345"
        val cases = listOf(
            Triple("the page asked for, reported without its code", "http://192.168.1.20:7777/kitchen", asked),
            Triple("the page asked for, as it was asked", asked, asked),
            Triple("another page of the PC's", "http://192.168.1.20:7777/", "http://192.168.1.20:7777/"),
            Triple("the page asked for, at a new address", "http://192.168.1.21:7777/kitchen", "http://192.168.1.21:7777/kitchen"),
        )
        for ((name, failed, want) in cases) assertEquals(name, want, retryOf(failed, asked))
        assertEquals("nothing asked yet", "http://192.168.1.20:7777/", retryOf("http://192.168.1.20:7777/", null))
    }

    @Test
    fun `the PC's pages are its scheme, host and port`() {
        val pc = "http://192.168.1.20:7777/#code=ABCD-2345"
        val same = listOf(
            "http://192.168.1.20:7777/kitchen",
            "HTTP://192.168.1.20:7777/pos?x=1",
            "http://192.168.1.20:7777",
        )
        for (url in same) assertTrue(url, samePc(url, pc))
        val other = listOf(
            "https://192.168.1.20:7777/",
            "http://192.168.1.21:7777/",
            "http://192.168.1.20:7778/",
            "http://192.168.1.20/",
            "http://mez.local:7777/",
            "https://mezpos.com/",
            AppPages.unreachable(pc),
            "about:blank",
            "",
        )
        for (url in other) assertFalse(url, samePc(url, pc))
        assertTrue("a default port, written or not", samePc("http://mez.local:80/kitchen", "http://mez.local/"))
        assertTrue("a host's case", samePc("http://MEZ.local/", "http://mez.local/"))
        assertFalse("no PC at all", samePc("http://mez.local/", ""))
    }

    @Test
    fun `a page takes the WebView to the PC's pages, and the app's from the app's`() {
        val pc = "http://192.168.1.20:7777/"
        val till = "http://192.168.1.20:7777/pos"
        val ask = AppPages.page("ask")
        val cases = listOf(
            Triple("the till to the kitchen display", "http://192.168.1.20:7777/kitchen" to till, true),
            Triple("the till to the back office", "http://192.168.1.20:7777/backoffice/" to till, true),
            Triple("the till to the internet", "https://mezpos.com/help" to till, false),
            Triple("the till to another device", "http://192.168.1.1/" to till, false),
            Triple("the till to the PC by another name", "http://mez.local:7777/" to till, false),
            Triple("the till to one of the app's pages", AppPages.unreachable(pc) to till, false),
            Triple("one of the app's pages to another", AppPages.page("settings") to ask, true),
            Triple("one of the app's pages to the internet", "https://example.com/" to ask, false),
            Triple("nothing yet to the app's page", ask to null, false),
            Triple("a scheme WebView refuses itself", "intent://scan/#Intent;end" to till, true),
            Triple("a blob of the PC's", "blob:http://192.168.1.20:7777/1234" to till, true),
        )
        for ((name, step, want) in cases) {
            val (url, from) = step
            assertEquals(name, want, mayOpen(url, from, pc))
        }
    }

    @Test
    fun `the health check is on the PC the page came from`() {
        val cases = mapOf(
            "http://192.168.1.20:7777/#code=ABCD-2345" to "http://192.168.1.20:7777/api/v1/health",
            "http://192.168.1.20:7777/kitchen?station=bar" to "http://192.168.1.20:7777/api/v1/health",
            "http://mez.local/backoffice/" to "http://mez.local/api/v1/health",
            "https://10.0.0.5:7443/" to "https://10.0.0.5:7443/api/v1/health",
            "http://[fd00::5]:7777/" to "http://[fd00::5]:7777/api/v1/health",
            "http://user:secret@192.168.1.20:7777/" to "http://192.168.1.20:7777/api/v1/health",
        )
        for ((page, want) in cases) assertEquals(page, want, healthOf(page))
    }

    @Test
    fun `where the app's page says it is looking`() {
        assertEquals("192.168.1.20:7777", place("http://192.168.1.20:7777/#code=ABCD-2345"))
        assertEquals("mez.local", place("http://mez.local/kitchen"))
        assertEquals("[fd00::5]:7777", place("http://[fd00::5]:7777/"))
    }

    @Test
    fun `no PC in an address that is not one`() {
        val others = listOf(
            "",
            "about:blank",
            "mez-app:retry",
            "file:///sdcard/till.html",
            "javascript:alert(1)",
            "http://",
            "http:///kitchen",
            "http://192.168.1.20:7777/ till",
            AppPages.ASSETS.replace("https:", "content:"),
        )
        for (url in others) {
            assertNull("health for $url", healthOf(url))
            assertNull("place for $url", place(url))
        }
    }
}

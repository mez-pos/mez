package com.mezpos.meztablet

import org.junit.Assert.assertEquals
import org.junit.Test

class BackTest {

    private val till = "http://192.168.1.20:7777/pos"
    private val tables = "http://192.168.1.20:7777/tables"
    private val ask = AppPages.page("ask")
    private val unreachable = AppPages.unreachable(till)

    private data class Case(
        val name: String,
        val current: String?,
        val behind: List<String?>,
        val canGoBack: Boolean,
        val want: Boolean,
    )

    @Test
    fun `Back goes back between the PC's pages, and nowhere else`() {
        val cases = listOf(
            Case("from tables to the till", tables, listOf(till), canGoBack = true, want = true),
            Case("further back, past an entry WebView skips", tables, listOf(till, till), canGoBack = true, want = true),
            Case("the first screen: Android's Back would leave the app", till, emptyList(), canGoBack = false, want = false),
            Case("only entries WebView skips behind", tables, listOf(till), canGoBack = false, want = false),
            Case("the page that asked for the local network behind", till, listOf(ask), canGoBack = true, want = false),
            Case("the app's page behind, past the PC's", tables, listOf(ask, till), canGoBack = true, want = false),
            Case("on can't reach the PC", unreachable, listOf(till), canGoBack = true, want = false),
            Case("can't reach the PC behind", tables, listOf(till, unreachable, till), canGoBack = true, want = false),
            Case("nothing loaded yet", null, emptyList(), canGoBack = false, want = false),
            Case("nothing loaded, an entry behind", null, listOf(till), canGoBack = true, want = false),
            Case("an entry with no address behind", tables, listOf(null, till), canGoBack = true, want = true),
        )
        for (c in cases) assertEquals(c.name, c.want, goesBack(c.current, c.behind, c.canGoBack))
    }
}

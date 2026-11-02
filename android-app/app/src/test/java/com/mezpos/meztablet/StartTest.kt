package com.mezpos.meztablet

import org.junit.Assert.assertEquals
import org.junit.Test

class StartTest {

    private data class Case(
        val name: String,
        val want: Start,
        val address: String = "http://192.168.1.20:7777/",
        val sdk: Int = ANDROID_17,
        val granted: Boolean = false,
        val askedBefore: Boolean = false,
        val androidWillAsk: Boolean = false,
    )

    @Test
    fun `where the app starts`() {
        val cases = listOf(
            Case("no address, even with the permission", Start.PAIR, address = "", granted = true),
            Case("no address on an older Android", Start.PAIR, address = "", sdk = 33),
            Case("no address, before Android 17 asks", Start.PAIR, address = ""),
            Case("no address, refused past asking", Start.PAIR, address = "", askedBefore = true),
            Case("Android 16 has no such permission", Start.MEZ, sdk = 36),
            Case("Android 10, the oldest the app runs on", Start.MEZ, sdk = 29),
            Case("Android 16, though asked once on another", Start.MEZ, sdk = 36, askedBefore = true),
            Case("granted", Start.MEZ, granted = true, askedBefore = true),
            Case("granted on a later Android", Start.MEZ, sdk = 38, granted = true),
            Case("granted without the app asking, by the group", Start.MEZ, granted = true),
            Case("first run on Android 17", Start.ASK),
            Case("first run on a later Android", Start.ASK, sdk = 38),
            Case("refused once, Android will ask again", Start.REFUSED, askedBefore = true, androidWillAsk = true),
            Case("refused before the app kept its note", Start.REFUSED, androidWillAsk = true),
            Case("refused past asking", Start.SETTINGS, askedBefore = true),
        )
        for (c in cases) {
            val got = start(c.address, c.sdk, c.granted, c.askedBefore, c.androidWillAsk)
            assertEquals(c.name, c.want, got)
        }
    }
}

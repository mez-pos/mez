package com.mezpos.meztablet

import org.junit.Assert.assertTrue
import org.junit.Assume.assumeTrue
import org.junit.Test

/**
 * The app's own query against a real mez's announcer (pion/mdns), over the
 * real network: skipped unless `MEZ_MDNS_LIVE` names the address the PC
 * should answer with. Run from the PC with mez running:
 *
 *     MEZ_MDNS_LIVE=192.168.1.45 ./gradlew testDebugUnitTest --tests '*MdnsLiveTest*' --rerun
 */
class MdnsLiveTest {

    @Test
    fun `a running mez answers the app's question for mez local`() {
        val expected = System.getenv("MEZ_MDNS_LIVE")
        assumeTrue("MEZ_MDNS_LIVE not set: no mez to ask", !expected.isNullOrEmpty())
        val found = Finder().find()
        assertTrue("asked mez.local, heard $found, not $expected", expected in found)
    }
}

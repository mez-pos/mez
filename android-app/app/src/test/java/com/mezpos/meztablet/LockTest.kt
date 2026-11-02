package com.mezpos.meztablet

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class LockTest {

    @Test
    fun `a device owner's tablet locks once the till has shown, and stays locked`() {
        assertTrue(shouldLock(owner = true, engaged = true, already = false, left = false, askedToPin = false))
        assertTrue("asked or not, a kiosk locks again", shouldLock(owner = true, engaged = true, already = false, left = false, askedToPin = true))
        assertFalse("before the till first shows: Android's prompts and pairing come first", shouldLock(owner = true, engaged = false, already = false, left = false, askedToPin = false))
        assertFalse("locked already", shouldLock(owner = true, engaged = true, already = true, left = false, askedToPin = false))
        assertFalse("left by the way out, until mez starts again", shouldLock(owner = true, engaged = true, already = false, left = true, askedToPin = false))
    }

    @Test
    fun `any other tablet is asked to pin once a start`() {
        assertTrue(shouldLock(owner = false, engaged = true, already = false, left = false, askedToPin = false))
        assertFalse("unpinned by Android's gesture, or refused: not asked again", shouldLock(owner = false, engaged = true, already = false, left = false, askedToPin = true))
        assertFalse(shouldLock(owner = false, engaged = false, already = false, left = false, askedToPin = false))
    }

    @Test
    fun `how a locked tablet is held, for its pages`() {
        assertEquals(Locking.KIOSK, lockingOf(owner = true, inLockTask = true))
        assertEquals(Locking.PINNED, lockingOf(owner = false, inLockTask = true))
        assertNull("not locked", lockingOf(owner = true, inLockTask = false))
    }

    @Test
    fun `leaving mez opens the tablet's own home app`() {
        val mez = HomeApp("com.mezpos.meztablet", "com.mezpos.meztablet.MainActivity", 0)
        val fallback = HomeApp("com.android.settings", "com.android.settings.FallbackHome", -1000)
        val pixel = HomeApp("com.google.android.apps.nexuslauncher", "com.google.android.apps.nexuslauncher.NexusLauncherActivity", 0)
        assertEquals(pixel, otherHome(listOf(mez, fallback, pixel), self = mez.pkg))
        assertNull("mez and Android's fallback alone: no home of the tablet's own", otherHome(listOf(mez, fallback), self = mez.pkg))
        val vendor = HomeApp("com.vendor.launcher", "com.vendor.launcher.Home", 10)
        assertEquals("the higher priority", vendor, otherHome(listOf(pixel, vendor), self = mez.pkg))
    }

    @Test
    fun `the pages say how the tablet is held`() {
        assertTrue(AppPages.unreachable("http://192.168.1.45:7777/", Locking.KIOSK).contains("lock=kiosk"))
        assertFalse(AppPages.unreachable("http://192.168.1.45:7777/", null).contains("lock="))
        assertTrue(AppPages.wayOutOpen("Priya", Locking.PINNED).contains("lock=pinned"))
        assertTrue(AppPages.wayOutOpen("Priya", Locking.PINNED).endsWith("#way-out-open"))
    }

    @Test
    fun `the lock's own links`() {
        val own = AppPages.page(AppPages.UNREACHABLE)
        assertEquals(Action.WIFI, Action.of("mez-app:wifi", from = own))
        assertEquals(Action.WAY_OUT_LEAVE, Action.of("mez-app:way-out-leave", from = own))
        assertEquals(Action.WAY_OUT_RELEASE, Action.of("mez-app:way-out-release", from = own))
        assertNull("not from a page of the PC's", Action.of("mez-app:way-out-release", from = "http://192.168.1.45:7777/"))
    }
}

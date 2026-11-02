package com.mezpos.meztablet

import android.app.admin.DeviceAdminReceiver

/*
 * The tablet locked to mez (ADR-419, ADR-420): lock task mode when the app is
 * the tablet's device owner, screen pinning when it is not. The lock waits
 * for the till to show once, so that Android's permission prompts and the
 * first pairing come before it, and from then on holds on every page of the
 * app's while mez is in front. The way out lifts it.
 */

/** How a locked tablet is held to mez. */
enum class Locking(val id: String) {
    /** Lock task mode as the device owner: home, recents and the shade do nothing. */
    KIOSK("kiosk"),

    /** Screen pinning: Android asked to pin, and its own gesture unpins. */
    PINNED("pinned"),
}

/**
 * Whether to start the lock now. [owner]: the app is the device owner.
 * [engaged]: the till has shown on this tablet since it was last released.
 * [already]: locked now. [left]: lifted by the way out, until mez has gone
 * behind another app and come back. [askedToPin]: screen pinning asked for
 * this start already: one refused, or undone by Android's gesture, is not
 * asked again until the next.
 */
fun shouldLock(owner: Boolean, engaged: Boolean, already: Boolean, left: Boolean, askedToPin: Boolean): Boolean =
    engaged && !already && !left && (owner || !askedToPin)

/** How the tablet is held now, or null if it is not locked. */
fun lockingOf(owner: Boolean, inLockTask: Boolean): Locking? = when {
    !inLockTask -> null
    owner -> Locking.KIOSK
    else -> Locking.PINNED
}

/** A home app on the tablet: its package, its activity, and its filter's priority. */
data class HomeApp(val pkg: String, val activity: String, val priority: Int)

/**
 * The tablet's own home app, for "Leave mez" to open: not mez, and not
 * Android's fallback, which claims a negative priority and shows only while
 * the tablet starts. Null when there is none.
 */
fun otherHome(homes: List<HomeApp>, self: String): HomeApp? =
    homes.filter { it.pkg != self && it.priority >= 0 }.maxByOrNull { it.priority }

/**
 * The app's device admin, which lets it become the tablet's device owner:
 * `adb shell dpm set-device-owner com.mezpos.meztablet/.Admin`, on a tablet
 * with no accounts (ADR-420). It holds no policy of its own; the lock is the
 * activity's.
 */
class Admin : DeviceAdminReceiver()

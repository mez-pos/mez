package com.mezpos.meztablet

/** Android 17, the first Android with the local network permission. */
const val ANDROID_17 = 37

/**
 * Android 17's local network permission, written out: with minSdk at 29,
 * `Manifest.permission.ACCESS_LOCAL_NETWORK` is a constant most tablets lack.
 * WebView traffic needs it too: it inherits the app's grant.
 */
const val LOCAL_NETWORK = "android.permission.ACCESS_LOCAL_NETWORK"

/** What the app shows: the mez PC's screens, or one of the app's own pages. */
enum class Start(val page: String?) {
    /** The mez PC's screens, at the address the app has. */
    MEZ(null),

    /** Why Android is about to ask for the local network, before it asks. */
    ASK("ask"),

    /** Refused, and Android will still ask again. */
    REFUSED("refused"),

    /** Refused past asking: only Android's Settings can allow it now. */
    SETTINGS("settings"),

    /** No address for the PC: a release build, until it pairs by scanning. */
    NO_ADDRESS("no-address"),
}

/**
 * Where the app starts, and where it goes on coming back to the front (from
 * Android's permission prompt, or from its Settings).
 *
 * [androidWillAsk] is `shouldShowRequestPermissionRationale`: true after a
 * refusal Android will still ask past, and false both before the first ask
 * and once it will not ask again. [askedBefore], the app's own note that it
 * has asked, tells those two apart.
 */
fun start(
    address: String,
    sdk: Int,
    granted: Boolean,
    askedBefore: Boolean,
    androidWillAsk: Boolean,
): Start = when {
    address.isEmpty() -> Start.NO_ADDRESS
    sdk < ANDROID_17 || granted -> Start.MEZ
    androidWillAsk -> Start.REFUSED
    !askedBefore -> Start.ASK
    else -> Start.SETTINGS
}

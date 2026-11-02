import java.net.URI
import javax.inject.Inject

plugins {
    alias(libs.plugins.android.application)
}

// An address a debug build starts from, as if "Pair this tablet" had been
// given it (ADR-414): for walking the app against a PC without pairing it
// each time. The emulator reaches the computer it runs on at 10.0.2.2:
//   ./gradlew assembleDebug -PmezUrl=http://10.0.2.2:7777/
// Without one, a debug build starts on "Pair this tablet", as a release does.
val mezUrl: String = providers.gradleProperty("mezUrl").getOrElse("")
if (mezUrl.isNotEmpty()) {
    // It becomes a Java string, and `make android` passes it in single quotes.
    require(Regex("""^https?://[^\s"'\\$]+$""").matches(mezUrl)) {
        "mezUrl \"$mezUrl\": want an http:// or https:// address, with no spaces, quotes, \\ or \$"
    }
    // The app knows the PC's pages by host and port, read as java.net.URI
    // reads them (Reach.kt): a host it cannot read, as a name with an
    // underscore, would be no PC's, and every page refused.
    require(!URI(mezUrl).host.isNullOrEmpty()) { "mezUrl \"$mezUrl\": no host in it that the app can read" }
}

// Where a debug build sends its question for mez.local (ADR-416, Task 5c):
// a responder's IPv4 address and port, for walking the app on an emulator,
// whose network carries no multicast. Without one, the group, as a release.
//   ./gradlew assembleDebug -PmezMdns=10.0.2.2:5354
val mezMdns: String = providers.gradleProperty("mezMdns").getOrElse("")
require(mezMdns.isEmpty() || Regex("""^\d{1,3}(\.\d{1,3}){3}:\d{1,5}$""").matches(mezMdns)) {
    "mezMdns \"$mezMdns\": want an IPv4 address and a port, as 10.0.2.2:5354"
}

// The version (ADR-410): `make android-release` passes mez's version as git
// describe writes it, and the build number, the commit count of main, as
// Android's code, so a tablet's app and its PC are named by the same tag.
// Without them a build is "dev", code 1.
val mezVersion: String = providers.gradleProperty("mezVersion").getOrElse("dev")
require(Regex("""^[0-9A-Za-z][0-9A-Za-z.+-]{0,63}$""").matches(mezVersion)) {
    "mezVersion \"$mezVersion\": want a version as git describe writes it"
}
val mezVersionCode: Int = providers.gradleProperty("mezVersionCode").getOrElse("1").let { raw ->
    requireNotNull(raw.toIntOrNull()?.takeIf { it in 1..2_100_000_000 }) {
        "mezVersionCode \"$raw\": want a whole number from 1 to 2100000000"
    }
}

// The release key, named by the environment and never kept in the
// repository (ADR-421): the keystore's path, its password, the key's alias
// and the key's password. Without all four a release build is not signed,
// and `make android-release` refuses to make one: a tablet that installed a
// package signed with any other key could not take the real one without
// uninstalling it, which costs its pairing.
val releaseKey: Map<String, String> = listOf(
    "MEZ_ANDROID_KEYSTORE",
    "MEZ_ANDROID_KEYSTORE_PASSWORD",
    "MEZ_ANDROID_KEY_ALIAS",
    "MEZ_ANDROID_KEY_PASSWORD",
).associateWith { providers.environmentVariable(it).getOrElse("") }
val releaseSigned = releaseKey.values.all { it.isNotEmpty() }

android {
    namespace = "com.mezpos.meztablet"
    compileSdk {
        version = release(37)
    }

    defaultConfig {
        applicationId = "com.mezpos.meztablet"
        minSdk = 29
        targetSdk = 37
        versionCode = mezVersionCode
        versionName = mezVersion

        // A release build starts from no address: "Pair this tablet".
        buildConfigField("String", "MEZ_URL", "\"\"")
        // And asks the group for mez.local.
        buildConfigField("String", "MEZ_MDNS", "\"\"")
    }

    buildFeatures {
        buildConfig = true
    }

    signingConfigs {
        if (releaseSigned) {
            create("release") {
                storeFile = file(releaseKey.getValue("MEZ_ANDROID_KEYSTORE"))
                storePassword = releaseKey.getValue("MEZ_ANDROID_KEYSTORE_PASSWORD")
                keyAlias = releaseKey.getValue("MEZ_ANDROID_KEY_ALIAS")
                keyPassword = releaseKey.getValue("MEZ_ANDROID_KEY_PASSWORD")
            }
        }
    }

    buildTypes {
        debug {
            buildConfigField("String", "MEZ_URL", "\"$mezUrl\"")
            buildConfigField("String", "MEZ_MDNS", "\"$mezMdns\"")
        }
        release {
            optimization {
                enable = true
            }
            if (releaseSigned) {
                signingConfig = signingConfigs.getByName("release")
            }
        }
    }
    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_11
        targetCompatibility = JavaVersion.VERSION_11
    }

    lint {
        abortOnError = true
        warningsAsErrors = true
        // A newer library or SDK is Dependabot's to raise, not a reason for
        // the build to fail on a day nothing here changed.
        disable += setOf("GradleDependency", "AndroidGradlePluginVersion", "NewerVersionAvailable", "OldTargetApi")
    }
}

// What the app's own pages borrow from the web app: two fonts with their
// licences, and the mark. Copied at build time so the repository holds one
// copy of each (tokens.md, Logo: never redrawn).
abstract class WebShared : DefaultTask() {
    @get:InputDirectory
    @get:PathSensitive(PathSensitivity.RELATIVE)
    abstract val fonts: DirectoryProperty

    @get:InputDirectory
    @get:PathSensitive(PathSensitivity.RELATIVE)
    abstract val brand: DirectoryProperty

    @get:OutputDirectory
    abstract val output: DirectoryProperty

    @get:Inject
    abstract val files: FileSystemOperations

    @TaskAction
    fun copy() {
        files.sync {
            from(fonts) {
                include(FONTS)
                into("fonts")
            }
            from(brand) {
                include(MARK)
                into("brand")
            }
            into(output)
        }
        // An include that matches nothing copies nothing and says nothing:
        // a font renamed in web/shared would ship a page without it.
        val wanted = FONTS.map { "fonts/$it" } + "brand/$MARK"
        val missing = wanted.filterNot { output.file(it).get().asFile.isFile }
        if (missing.isNotEmpty()) {
            throw GradleException("web/shared has no ${missing.joinToString()}: start.html needs them")
        }
    }

    private companion object {
        val FONTS = listOf("fraunces-600.woff2", "dm-sans-14.woff2", "OFL-fraunces.txt", "OFL-dmsans.txt")
        const val MARK = "mez-mark.svg"
    }
}

val webShared = tasks.register<WebShared>("webShared") {
    val shared = layout.projectDirectory.dir("../../web/shared")
    fonts.set(shared.dir("fonts"))
    brand.set(shared.dir("brand"))
    output.set(layout.buildDirectory.dir("generated/webShared"))
}

androidComponents {
    onVariants { variant ->
        variant.sources.assets?.addGeneratedSourceDirectory(webShared, WebShared::output)
    }
}

// AppPagesTest reads the app's page to hold its sections to Start's pages:
// a change to the page must run the tests again, not find them up to date.
tasks.withType<Test>().configureEach {
    inputs.file("src/main/assets/start.html").withPathSensitivity(PathSensitivity.RELATIVE)
}

dependencies {
    implementation(libs.androidx.activity)
    // The camera for "Pair this tablet" (ADR-415): CameraX without its
    // camera-view, which would bring AppCompat back (ADR-412) for one view.
    implementation(libs.androidx.camera.camera2)
    implementation(libs.androidx.camera.core)
    implementation(libs.androidx.camera.lifecycle)
    implementation(libs.androidx.core.ktx)
    implementation(libs.androidx.webkit)
    // QR codes read in plain Java: no Play services.
    implementation(libs.zxing.core)
    testImplementation(libs.junit)
}

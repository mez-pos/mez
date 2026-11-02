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

android {
    namespace = "com.mezpos.meztablet"
    compileSdk {
        version = release(37)
    }

    defaultConfig {
        applicationId = "com.mezpos.meztablet"
        minSdk = 29
        targetSdk = 37
        versionCode = 1
        versionName = "1.0"

        // A release build starts from no address: "Pair this tablet".
        buildConfigField("String", "MEZ_URL", "\"\"")
    }

    buildFeatures {
        buildConfig = true
    }

    buildTypes {
        debug {
            buildConfigField("String", "MEZ_URL", "\"$mezUrl\"")
        }
        release {
            optimization {
                enable = true
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

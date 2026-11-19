plugins {
    `maven-publish`
    signing
}

group = "io.hinkal"
version = providers.gradleProperty("sdkVersion").get()

val distDir = layout.projectDirectory.dir("../../../../dist/go/android")
val aarFile = distDir.file("hinkal.aar")
val sourcesJarFile = distDir.file("hinkal-sources.jar")

val javadocJar by tasks.registering(Jar::class) {
    archiveClassifier.set("javadoc")
    // Central Portal requires a javadoc artifact even for AAR-only gomobile output.
}

publishing {
    publications {
        create<MavenPublication>("release") {
            groupId = project.group.toString()
            artifactId = "hinkal-android"
            version = project.version.toString()

            artifact(aarFile) {
                extension = "aar"
            }
            artifact(sourcesJarFile) {
                classifier = "sources"
                extension = "jar"
            }
            artifact(javadocJar)

            pom {
                name.set("Hinkal Android SDK")
                description.set("Android bindings for the Hinkal Protocol Go SDK.")
                url.set("https://github.com/Hinkal-Protocol/Hinkal-Protocol")
                licenses {
                    license {
                        name.set("MIT License")
                        url.set("https://github.com/Hinkal-Protocol/Hinkal-Protocol/blob/main/LICENSE")
                    }
                }
                developers {
                    developer {
                        id.set("hinkal-protocol")
                        name.set("Hinkal Protocol")
                        url.set("https://hinkal.pro")
                    }
                }
                scm {
                    connection.set("scm:git:https://github.com/Hinkal-Protocol/Hinkal-Protocol.git")
                    developerConnection.set("scm:git:git@github.com:Hinkal-Protocol/Hinkal-Protocol.git")
                    url.set("https://github.com/Hinkal-Protocol/Hinkal-Protocol")
                }
            }
        }
    }

    repositories {
        maven {
            name = "LocalStaging"
            url = uri(layout.buildDirectory.dir("staging-deploy"))
        }
    }
}

signing {
    val gpgSigningKey = providers.environmentVariable("GPG_SIGNING_KEY")
    val gpgSigningPassword = providers.environmentVariable("GPG_SIGNING_PASSWORD")
    useInMemoryPgpKeys(gpgSigningKey.orNull, gpgSigningPassword.orNull)
    sign(publishing.publications["release"])
}

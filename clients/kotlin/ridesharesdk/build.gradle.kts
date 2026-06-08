plugins { kotlin("jvm") }

// Depends on ARES-core's ares-client-fhe module. For local development use a
// composite build: add `includeBuild("../../../../ARES-core/clients/kotlin")`
// to settings.gradle.kts. For production, consume via published artifact:
//   implementation("com.github.Fheyalabs:ARES-core:latest.release")
dependencies {
    // Replace with the appropriate ARES-core dependency for your setup.
    // implementation(project(":ares-client-fhe"))
    testImplementation(kotlin("test"))
    testImplementation("org.junit.jupiter:junit-jupiter:5.10.2")
}
kotlin { jvmToolchain(17) }
tasks.test { useJUnitPlatform() }

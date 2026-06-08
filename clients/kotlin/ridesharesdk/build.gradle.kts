plugins {
    kotlin("jvm")
}

dependencies {
    implementation(project(":ares-client-fhe"))
    testImplementation(kotlin("test"))
    testImplementation("org.junit.jupiter:junit-jupiter:5.10.2")
}
kotlin { jvmToolchain(17) }
tasks.test {
    useJUnitPlatform()
    environment("ARES_FHE_ALLOW_INSECURE", "1")
    systemProperty("java.library.path",
        layout.buildDirectory.dir("native").get().asFile.absolutePath +
        File.pathSeparator + System.getProperty("java.library.path"))
}

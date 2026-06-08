pluginManagement {
    repositories {
        gradlePluginPortal()
        mavenCentral()
        google()
    }
}
dependencyResolutionManagement {
    repositories {
        mavenCentral()
        google()
    }
}
rootProject.name = "rideshare-kotlin-client"
includeBuild("../../../ARES-core/clients/kotlin")
include("ridesharesdk")

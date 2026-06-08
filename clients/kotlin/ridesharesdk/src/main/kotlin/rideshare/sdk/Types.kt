package rideshare.sdk

data class Keypair(val publicKey: ByteArray, val secretKey: ByteArray) {
    override fun equals(other: Any?): Boolean =
        other is Keypair && publicKey.contentEquals(other.publicKey) && secretKey.contentEquals(other.secretKey)
    override fun hashCode(): Int = publicKey.contentHashCode() * 31 + secretKey.contentHashCode()
}

data class EncryptedBid(val ciphertext: ByteArray, val priceCents: Int, val nonce: ByteArray) {
    override fun equals(other: Any?): Boolean =
        other is EncryptedBid && ciphertext.contentEquals(other.ciphertext) && priceCents == other.priceCents && nonce.contentEquals(other.nonce)
    override fun hashCode(): Int = ciphertext.contentHashCode() * 31 + priceCents
}

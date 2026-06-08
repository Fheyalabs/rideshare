package rideshare.sdk

import ares.client.fhe.CryptoContext
import kotlin.random.Random

object Driver {
    fun encryptBid(
        priceCents: Int, pkBytes: ByteArray,
        nonce: ByteArray = Random.nextBytes(16),
        ringDim: Int = 1 shl 15, depth: Int = 5,
        scalingFactor: Double = Math.scalb(1.0, 50)
    ): EncryptedBid {
        val ctx = CryptoContext(ringDim, scalingFactor, depth)
        ctx.use {
            val pk = ctx.deserializePublicKey(pkBytes)
            val ct = ctx.encrypt(doubleArrayOf(priceCents.toDouble(), 0.0, 0.0, 0.0), pk)
            return EncryptedBid(ctx.serialize(ct), priceCents, nonce)
        }
    }
}

package rideshare.sdk

import ares.client.fhe.CryptoContext

object Rider {
    fun keygen(
        ringDim: Int = 1 shl 15, depth: Int = 5,
        scalingFactor: Double = Math.scalb(1.0, 50)
    ): Keypair {
        val ctx = CryptoContext(ringDim, scalingFactor, depth)
        ctx.use {
            val kp = ctx.singleKeyGen()
            return Keypair(ctx.serialize(kp.publicKey), ctx.serialize(kp.secretKey))
        }
    }

    fun decryptMasks(
        skBytes: ByteArray, encryptedMasks: List<ByteArray>,
        ringDim: Int = 1 shl 15, depth: Int = 5,
        scalingFactor: Double = Math.scalb(1.0, 50)
    ): Pair<Int, DoubleArray> {
        val ctx = CryptoContext(ringDim, scalingFactor, depth)
        ctx.use {
            val sk = ctx.deserializeSecretKeyShare(skBytes, true)
            val masks = DoubleArray(encryptedMasks.size)
            var best = -1; var bestVal = 0.0
            for ((i, ctBytes) in encryptedMasks.withIndex()) {
                val ct = ctx.deserializeCiphertext(ctBytes)
                val vals = ctx.decryptSingle(ct, sk, 1)
                masks[i] = vals[0]
                if (best < 0 || vals[0] > bestVal) { best = i; bestVal = vals[0] }
            }
            return Pair(best, masks)
        }
    }
}

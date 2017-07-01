// See LICENSE for licensing information

TEXT ·huffasm(SB),$0-56
	// move all arguments into registers as per the ARM64 calling convention
	MOVD state+0(FP), R0
	MOVD buffer+8(FP), R1
	MOVD block+16(FP), R2
	MOVD last_dc_val+24(FP), R3
	MOVD dctbl+32(FP), R4
	MOVD actbl+40(FP), R5
	BL jsimd_huff_encode_one_block_neon_slowtbl(SB)
	MOVD R0, ret+48(FP)
	RET


// See LICENSE for licensing information

#include "textflag.h"

// Ljsimd_extrgb_ycc_neon_consts:
// .short 19595, 38470, 7471, 11059
DATA extrgb_ycc_consts<>+0x00(SB)/4, $0x96464c9b
DATA extrgb_ycc_consts<>+0x04(SB)/4, $0x2b331d2f
// .short 21709, 32768, 27439, 5329
DATA extrgb_ycc_consts<>+0x08(SB)/4, $0x800054cd
DATA extrgb_ycc_consts<>+0x0c(SB)/4, $0x14d16b2f
// .short 32767, 128, 32767, 128
DATA extrgb_ycc_consts<>+0x10(SB)/4, $0x00807fff
DATA extrgb_ycc_consts<>+0x14(SB)/4, $0x00807fff
// .short 32767, 128, 32767, 128
DATA extrgb_ycc_consts<>+0x18(SB)/4, $0x00807fff
DATA extrgb_ycc_consts<>+0x1c(SB)/4, $0x00807fff
GLOBL extrgb_ycc_consts<>(SB), RODATA, $32

TEXT ·rgbyccconvertasm(SB),$0-32
	// move all arguments into registers as per the ARM64 calling convention
	MOVW output_width+0(FP), R0
	MOVD input_buf+8(FP), R1
	MOVD output_buf+16(FP), R2
	MOVW output_row+24(FP), R3
	MOVW num_rows+28(FP), R4

// NB: register R11 is used by the Go linker to hold temporary values. need to be careful

	// adr x13, Ljsimd_extrgb_ycc_neon_consts
	MOVD $extrgb_ycc_consts<>(SB), R13

	// ld1             {v0.8h, v1.8h}, [x13]
	WORD $0x4c40a5a0

	// ldr x5, [x2]
	MOVD 0(R2), R5
	// ldr x6, [x2, #8]
	MOVD 8(R2), R6
	// ldr x2, [x2, #16]
	MOVD 16(R2), R2

	// TODO: can we skip saving the registers?
	// TODO: try RSP as register name
	// sub sp, sp, #64
	//WORD $0xd10103ff
	// mov x9, sp
	//WORD $0x910003e9
	// st1             {v8.8b, v9.8b, v10.8b, v11.8b}, [x9], 32
	//WORD $0x0c9f2128
	// st1             {v12.8b, v13.8b, v14.8b, v15.8b}, [x9], 32
	//WORD $0x0c9f212c

	// cmp w4, #0x1
	CMPW $1, R4
	// b.lt 9f
	BLT l9

l0:
	// gas: ldr Y, [OUTPUT_BUF0, OUTPUT_ROW, uxtw #3]
	// obj: ldr x9, [x5, w3, uxtw #3]
	WORD $0xf86358a9
	// gas: ldr U, [OUTPUT_BUF1, OUTPUT_ROW, uxtw #3]
	// obj: ldr x10, [x6, w3, uxtw #3]
	WORD $0xf86358ca
	// gas: mov N, OUTPUT_WIDTH
	// obj: mov w12, w0
	WORD $0x2a0003ec
	// gas: ldr V, [OUTPUT_BUF2, OUTPUT_ROW, uxtw #3]
	// obj: ldr x11, [x2, w3, uxtw #3]
	WORD $0xf863584b
	// gas: add OUTPUT_ROW, OUTPUT_ROW, #1
	// obj: add w3, w3, #0x1
	ADDW $1, R3, R3
	// gas: ldr RGB, [INPUT_BUF], #8
	// obj: ldr x7, [x1], #8
	WORD $0xf8408427

	// inner loop over pixels
	// gas: subs N, N, #8
	// obj: subs w12, w12, #0x8
	SUBSW $0x8, R12, R12
	// gas: b.lt 3f
	// obj: b.lt 11198
	BLT l3

	// do_load 8
#include "do_load8.inc"
	// do_rgb_to_yuv_stage1 (2-stage pipelined RGB→YCbCr conversion)
#include "do_rgb_to_yuv_stage1.inc"
	// gas: subs N, N, #8
	// obj: subs w12, w12, #0x8
	SUBSW $0x8, R12, R12
	// gas: b.lt 2f
	BLT l2
l1:
	// do_rgb_to_yuv_stage2_store_load_stage1
#include "do_rgb_to_yuv_stage2.inc"
	// do_load 8
#include "do_load8.inc"
	// gas: st1 {v20.8b}, [Y], #8
	// obj: st1 {v20.8b}, [x9], #8
	WORD $0x0c9f7134
	// gas: st1 {v21.8b}, [U], #8
	// obj: st1 {v21.8b}, [x10], #8
	WORD $0x0c9f7155
	// gas: st1 {v22.8b}, [V], #8
	// obj: st1 {v22.8b}, [x11], #8
	WORD $0x0c9f7176

#include "do_rgb_to_yuv_stage1.inc"

	// gas: subs N, N, #8
	// obj: subs    w12, w12, #0x8
	SUBSW $0x8, R12, R12

	// gas: b.ge 1b
	// obj: b.ge 110bc
	BGE l1
l2:
#include "do_rgb_to_yuv_stage2.inc"
#include "do_store8.inc"
	// gas: tst N, #7
	// obj: tst w12, #0x7
	WORD $0x7200099f
	// gas: b.eq 8f
	BEQ l8
l3:
	// gas: tbz             N, #2, 3f
	// obj: tbz     w12, #2, 111ac
	TBZ $2, R12, l33 // requires go1.9: https://go-review.googlesource.com/c/33594/
	// do_load 4
#include "do_load4.inc"
l33:
	// gas: tbz             N, #1, 4f
	// obj: tbz     w12, #1, 111b8
	TBZ $1, R12, l4
	// do_load 2
#include "do_load2.inc"
l4:
	// gas: tbz             N, #0, 5f
	// obj: tbz     w12, #0, 111c0
	TBZ $0, R12, l5
	// do_load 1
#include "do_load1.inc"
l5:
	// do_rgb_to_yuv
#include "do_rgb_to_yuv_stage1.inc"
#include "do_rgb_to_yuv_stage2.inc"
	// gas: tbz             N, #2, 6f
	// obj: tbz     w12, #2, 1127c
	TBZ $2, R12, l6
	// do_store 4
#include "do_store4.inc"
l6:
	// gas: tbz             N, #1, 7f
	// obj: tbz     w12, #1, 11298
	TBZ $1, R12, l7
	// do_store 2
#include "do_store2.inc"
l7:
	// gas: tbz             N, #0, 8f
	// obj: tbz     w12, #0, 112a8
	TBZ $0, R12, l8
	// do_store 1
#include "do_store1.inc"
l8:
	// gas: subs            NUM_ROWS, NUM_ROWS, #1
	// obj: subs    w4, w4, #0x1
	SUBSW $1, R4, R4
	// gas: b.gt            0b
	// obj: b.gt    11028
	BGT l0

l9:
	// TODO: restore registers
	RET

// See LICENSE for licensing information

#include "textflag.h"

// func convsampasm(sample_data **uint8, start_col uint32, workspace *uint16)
TEXT ·convsampasm(SB),$0-24
	MOVD sample_data+0(FP), R0
	MOVW start_col+8(FP), R1
	MOVD workspace+16(FP), R2

	WORD $0x2a0103e1 // mov     w1, w1
	WORD $0x52801003 // mov     w3, #0x80                       // #128
	WORD $0xa8c12809 // ldp     x9, x10, [x0], #16
	WORD $0xa8c1300b // ldp     x11, x12, [x0], #16
	WORD $0x0e010c60 // dup     v0.8b, w3
	WORD $0x8b010129 // add     x9, x9, x1
	WORD $0x8b01014a // add     x10, x10, x1
	WORD $0xa8c1380d // ldp     x13, x14, [x0], #16
	WORD $0x8b01016b // add     x11, x11, x1
	WORD $0x8b01018c // add     x12, x12, x1
	WORD $0xa8c1100f // ldp     x15, x4, [x0], #16
	WORD $0x8b0101ad // add     x13, x13, x1
	WORD $0x8b0101ce // add     x14, x14, x1
	WORD $0x0c407130 // ld1     {v16.8b}, [x9]
	WORD $0x8b0101ef // add     x15, x15, x1
	WORD $0x8b010084 // add     x4, x4, x1
	WORD $0x0c407151 // ld1     {v17.8b}, [x10]
	WORD $0x2e202210 // usubl   v16.8h, v16.8b, v0.8b
	WORD $0x0c407172 // ld1     {v18.8b}, [x11]
	WORD $0x2e202231 // usubl   v17.8h, v17.8b, v0.8b
	WORD $0x0c407193 // ld1     {v19.8b}, [x12]
	WORD $0x2e202252 // usubl   v18.8h, v18.8b, v0.8b
	WORD $0x0c4071b4 // ld1     {v20.8b}, [x13]
	WORD $0x2e202273 // usubl   v19.8h, v19.8b, v0.8b
	WORD $0x0c4071d5 // ld1     {v21.8b}, [x14]
	WORD $0x4c9f2450 // st1     {v16.8h-v19.8h}, [x2], #64
	WORD $0x2e202294 // usubl   v20.8h, v20.8b, v0.8b
	WORD $0x0c4071f6 // ld1     {v22.8b}, [x15]
	WORD $0x2e2022b5 // usubl   v21.8h, v21.8b, v0.8b
	WORD $0x0c407097 // ld1     {v23.8b}, [x4]
	WORD $0x2e2022d6 // usubl   v22.8h, v22.8b, v0.8b
	WORD $0x2e2022f7 // usubl   v23.8h, v23.8b, v0.8b
	WORD $0x4c9f2454 // st1     {v20.8h-v23.8h}, [x2], #64
	RET              // br      x30

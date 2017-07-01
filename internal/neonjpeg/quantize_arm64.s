// See LICENSE for licensing information

#include "textflag.h"

TEXT ·quantizeasm(SB),$0-24
	MOVD coef_block+0(FP), R0
	MOVD divisors+8(FP), R1
	MOVD workspace+16(FP), R2

	WORD $0xd280004b // mov     x11, #0x2                       // #2
	WORD $0x91020029 // add     x9, x1, #0x80
	WORD $0x9106002a // add     x10, x1, #0x180
l1:
	WORD $0xf100056b // subs    x11, x11, #0x1
	WORD $0x4cdf2440 // ld1     {v0.8h-v3.8h}, [x2], #64
	WORD $0x4cdf2524 // ld1     {v4.8h-v7.8h}, [x9], #64
	WORD $0x4e60b814 // abs     v20.8h, v0.8h
	WORD $0x4e60b835 // abs     v21.8h, v1.8h
	WORD $0x4e60b856 // abs     v22.8h, v2.8h
	WORD $0x4e60b877 // abs     v23.8h, v3.8h
	WORD $0x4cdf243c // ld1     {v28.8h-v31.8h}, [x1], #64
	WORD $0x4e648694 // add     v20.8h, v20.8h, v4.8h
	WORD $0x4e6586b5 // add     v21.8h, v21.8h, v5.8h
	WORD $0x4e6686d6 // add     v22.8h, v22.8h, v6.8h
	WORD $0x4e6786f7 // add     v23.8h, v23.8h, v7.8h
	WORD $0x2e7cc284 // umull   v4.4s, v20.4h, v28.4h
	WORD $0x6e7cc290 // umull2  v16.4s, v20.8h, v28.8h
	WORD $0x2e7dc2a5 // umull   v5.4s, v21.4h, v29.4h
	WORD $0x6e7dc2b1 // umull2  v17.4s, v21.8h, v29.8h
	WORD $0x2e7ec2c6 // umull   v6.4s, v22.4h, v30.4h
	WORD $0x6e7ec2d2 // umull2  v18.4s, v22.8h, v30.8h
	WORD $0x2e7fc2e7 // umull   v7.4s, v23.4h, v31.4h
	WORD $0x6e7fc2f3 // umull2  v19.4s, v23.8h, v31.8h
	WORD $0x4cdf2558 // ld1     {v24.8h-v27.8h}, [x10], #64
	WORD $0x0f108484 // shrn    v4.4h, v4.4s, #16
	WORD $0x0f1084a5 // shrn    v5.4h, v5.4s, #16
	WORD $0x0f1084c6 // shrn    v6.4h, v6.4s, #16
	WORD $0x0f1084e7 // shrn    v7.4h, v7.4s, #16
	WORD $0x4f108604 // shrn2   v4.8h, v16.4s, #16
	WORD $0x4f108625 // shrn2   v5.8h, v17.4s, #16
	WORD $0x4f108646 // shrn2   v6.8h, v18.4s, #16
	WORD $0x4f108667 // shrn2   v7.8h, v19.4s, #16
	WORD $0x6e60bb18 // neg     v24.8h, v24.8h
	WORD $0x6e60bb39 // neg     v25.8h, v25.8h
	WORD $0x6e60bb5a // neg     v26.8h, v26.8h
	WORD $0x6e60bb7b // neg     v27.8h, v27.8h
	WORD $0x4f110400 // sshr    v0.8h, v0.8h, #15
	WORD $0x4f110421 // sshr    v1.8h, v1.8h, #15
	WORD $0x4f110442 // sshr    v2.8h, v2.8h, #15
	WORD $0x4f110463 // sshr    v3.8h, v3.8h, #15
	WORD $0x6e784484 // ushl    v4.8h, v4.8h, v24.8h
	WORD $0x6e7944a5 // ushl    v5.8h, v5.8h, v25.8h
	WORD $0x6e7a44c6 // ushl    v6.8h, v6.8h, v26.8h
	WORD $0x6e7b44e7 // ushl    v7.8h, v7.8h, v27.8h
	WORD $0x6e201c84 // eor     v4.16b, v4.16b, v0.16b
	WORD $0x6e211ca5 // eor     v5.16b, v5.16b, v1.16b
	WORD $0x6e221cc6 // eor     v6.16b, v6.16b, v2.16b
	WORD $0x6e231ce7 // eor     v7.16b, v7.16b, v3.16b
	WORD $0x6e608484 // sub     v4.8h, v4.8h, v0.8h
	WORD $0x6e6184a5 // sub     v5.8h, v5.8h, v1.8h
	WORD $0x6e6284c6 // sub     v6.8h, v6.8h, v2.8h
	WORD $0x6e6384e7 // sub     v7.8h, v7.8h, v3.8h
	WORD $0x4c9f2404 // st1     {v4.8h-v7.8h}, [x0], #64
	BNE l1
	RET              // br      x30

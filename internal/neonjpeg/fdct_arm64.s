// See LICENSE for licensing information

#include "textflag.h"

DATA fdct_consts<>+0x0(SB)/4, $0xf384098e
DATA fdct_consts<>+0x4(SB)/4, $0x187e1151
DATA fdct_consts<>+0x8(SB)/4, $0x25a1e333
DATA fdct_consts<>+0xc(SB)/4, $0xc4df300b
DATA fdct_consts<>+0x10(SB)/4, $0x41b3c13b
DATA fdct_consts<>+0x14(SB)/4, $0x6254adfd
DATA fdct_consts<>+0x18(SB)/4, $0x00000000
DATA fdct_consts<>+0x1c(SB)/4, $0x00000000
GLOBL fdct_consts<>(SB), NOPTR+RODATA, $32

// func fdctasm(workspace *uint16)
TEXT ·fdctasm(SB),$0-8
	MOVD workspace+0(FP), R0

	MOVD $fdct_consts<>(SB), R9 // adr     x9, b380 <Ljsimd_fdct_islow_neon_consts>
	WORD $0x4c40a520 // ld1     {v0.8h, v1.8h}, [x9]
	WORD $0xd10103ff // sub     sp, sp, #0x40
	WORD $0x910003ea // mov     x10, sp
	WORD $0x0c9f2148 // st1     {v8.8b-v11.8b}, [x10], #32
	WORD $0x0c9f214c // st1     {v12.8b-v15.8b}, [x10], #32
	WORD $0x4cdf2410 // ld1     {v16.8h-v19.8h}, [x0], #64
	WORD $0x4c402414 // ld1     {v20.8h-v23.8h}, [x0]
	WORD $0xd1010000 // sub     x0, x0, #0x40
	WORD $0x4e512a1f // trn1    v31.8h, v16.8h, v17.8h
	WORD $0x4e532a42 // trn1    v2.8h, v18.8h, v19.8h
	WORD $0x4e552a83 // trn1    v3.8h, v20.8h, v21.8h
	WORD $0x4e572ac4 // trn1    v4.8h, v22.8h, v23.8h
	WORD $0x4e516a11 // trn2    v17.8h, v16.8h, v17.8h
	WORD $0x4e536a53 // trn2    v19.8h, v18.8h, v19.8h
	WORD $0x4e556a95 // trn2    v21.8h, v20.8h, v21.8h
	WORD $0x4e576ad7 // trn2    v23.8h, v22.8h, v23.8h
	WORD $0x4e842874 // trn1    v20.4s, v3.4s, v4.4s
	WORD $0x4e846864 // trn2    v4.4s, v3.4s, v4.4s
	WORD $0x4e822be3 // trn1    v3.4s, v31.4s, v2.4s
	WORD $0x4e826bf2 // trn2    v18.4s, v31.4s, v2.4s
	WORD $0x4e932a3f // trn1    v31.4s, v17.4s, v19.4s
	WORD $0x4e936a33 // trn2    v19.4s, v17.4s, v19.4s
	WORD $0x4e976aa2 // trn2    v2.4s, v21.4s, v23.4s
	WORD $0x4e972ab5 // trn1    v21.4s, v21.4s, v23.4s
	WORD $0x4ec46a56 // trn2    v22.2d, v18.2d, v4.2d
	WORD $0x4ed42870 // trn1    v16.2d, v3.2d, v20.2d
	WORD $0x4ed52bf1 // trn1    v17.2d, v31.2d, v21.2d
	WORD $0x4ec26a77 // trn2    v23.2d, v19.2d, v2.2d
	WORD $0x4ec42a52 // trn1    v18.2d, v18.2d, v4.2d
	WORD $0x4ed46874 // trn2    v20.2d, v3.2d, v20.2d
	WORD $0x4ec22a73 // trn1    v19.2d, v19.2d, v2.2d
	WORD $0x4ed56bf5 // trn2    v21.2d, v31.2d, v21.2d
	WORD $0x4e778618 // add     v24.8h, v16.8h, v23.8h
	WORD $0x6e77861f // sub     v31.8h, v16.8h, v23.8h
	WORD $0x4e768639 // add     v25.8h, v17.8h, v22.8h
	WORD $0x6e76863e // sub     v30.8h, v17.8h, v22.8h
	WORD $0x4e75865a // add     v26.8h, v18.8h, v21.8h
	WORD $0x6e75865d // sub     v29.8h, v18.8h, v21.8h
	WORD $0x4e74867b // add     v27.8h, v19.8h, v20.8h
	WORD $0x6e74867c // sub     v28.8h, v19.8h, v20.8h
	WORD $0x4e7b8708 // add     v8.8h, v24.8h, v27.8h
	WORD $0x6e7b8709 // sub     v9.8h, v24.8h, v27.8h
	WORD $0x4e7a872a // add     v10.8h, v25.8h, v26.8h
	WORD $0x6e7a872b // sub     v11.8h, v25.8h, v26.8h
	WORD $0x4e6a8510 // add     v16.8h, v8.8h, v10.8h
	WORD $0x6e6a8514 // sub     v20.8h, v8.8h, v10.8h
	WORD $0x4e698572 // add     v18.8h, v11.8h, v9.8h
	WORD $0x4f125610 // shl     v16.8h, v16.8h, #2
	WORD $0x4f125694 // shl     v20.8h, v20.8h, #2
	WORD $0x4f60a258 // smull2  v24.4s, v18.8h, v0.h[2]
	WORD $0x0f60a252 // smull   v18.4s, v18.4h, v0.h[2]
	WORD $0x4eb21e56 // mov     v22.16b, v18.16b
	WORD $0x4eb81f19 // mov     v25.16b, v24.16b
	WORD $0x0f702132 // smlal   v18.4s, v9.4h, v0.h[3]
	WORD $0x4f702138 // smlal2  v24.4s, v9.8h, v0.h[3]
	WORD $0x0f702976 // smlal   v22.4s, v11.4h, v0.h[7]
	WORD $0x4f702979 // smlal2  v25.4s, v11.8h, v0.h[7]
	WORD $0x0f158e52 // rshrn   v18.4h, v18.4s, #11
	WORD $0x0f158ed6 // rshrn   v22.4h, v22.4s, #11
	WORD $0x4f158f12 // rshrn2  v18.8h, v24.4s, #11
	WORD $0x4f158f36 // rshrn2  v22.8h, v25.4s, #11
	WORD $0x4e7f8788 // add     v8.8h, v28.8h, v31.8h
	WORD $0x4e7e87a9 // add     v9.8h, v29.8h, v30.8h
	WORD $0x4e7e878a // add     v10.8h, v28.8h, v30.8h
	WORD $0x4e7f87ab // add     v11.8h, v29.8h, v31.8h
	WORD $0x0f50a944 // smull   v4.4s, v10.4h, v0.h[5]
	WORD $0x4f50a945 // smull2  v5.4s, v10.8h, v0.h[5]
	WORD $0x0f502964 // smlal   v4.4s, v11.4h, v0.h[5]
	WORD $0x4f502965 // smlal2  v5.4s, v11.8h, v0.h[5]
	WORD $0x4f40a398 // smull2  v24.4s, v28.8h, v0.h[0]
	WORD $0x4f51a3b9 // smull2  v25.4s, v29.8h, v1.h[1]
	WORD $0x4f71a3da // smull2  v26.4s, v30.8h, v1.h[3]
	WORD $0x4f60abfb // smull2  v27.4s, v31.8h, v0.h[6]
	WORD $0x0f40a39c // smull   v28.4s, v28.4h, v0.h[0]
	WORD $0x0f51a3bd // smull   v29.4s, v29.4h, v1.h[1]
	WORD $0x0f71a3de // smull   v30.4s, v30.4h, v1.h[3]
	WORD $0x0f60abff // smull   v31.4s, v31.4h, v0.h[6]
	WORD $0x4f40a90c // smull2  v12.4s, v8.8h, v0.h[4]
	WORD $0x4f61a12d // smull2  v13.4s, v9.8h, v1.h[2]
	WORD $0x4f41a14e // smull2  v14.4s, v10.8h, v1.h[0]
	WORD $0x4f50a16f // smull2  v15.4s, v11.8h, v0.h[1]
	WORD $0x0f40a908 // smull   v8.4s, v8.4h, v0.h[4]
	WORD $0x0f61a129 // smull   v9.4s, v9.4h, v1.h[2]
	WORD $0x0f41a14a // smull   v10.4s, v10.4h, v1.h[0]
	WORD $0x0f50a16b // smull   v11.4s, v11.4h, v0.h[1]
	WORD $0x4ea4854a // add     v10.4s, v10.4s, v4.4s
	WORD $0x4ea585ce // add     v14.4s, v14.4s, v5.4s
	WORD $0x4ea4856b // add     v11.4s, v11.4s, v4.4s
	WORD $0x4ea585ef // add     v15.4s, v15.4s, v5.4s
	WORD $0x4ea8879c // add     v28.4s, v28.4s, v8.4s
	WORD $0x4eac8718 // add     v24.4s, v24.4s, v12.4s
	WORD $0x4ea987bd // add     v29.4s, v29.4s, v9.4s
	WORD $0x4ead8739 // add     v25.4s, v25.4s, v13.4s
	WORD $0x4eaa87de // add     v30.4s, v30.4s, v10.4s
	WORD $0x4eae875a // add     v26.4s, v26.4s, v14.4s
	WORD $0x4eab87ff // add     v31.4s, v31.4s, v11.4s
	WORD $0x4eaf877b // add     v27.4s, v27.4s, v15.4s
	WORD $0x4eaa879c // add     v28.4s, v28.4s, v10.4s
	WORD $0x4eae8718 // add     v24.4s, v24.4s, v14.4s
	WORD $0x4eab87bd // add     v29.4s, v29.4s, v11.4s
	WORD $0x4eaf8739 // add     v25.4s, v25.4s, v15.4s
	WORD $0x4ea987de // add     v30.4s, v30.4s, v9.4s
	WORD $0x4ead875a // add     v26.4s, v26.4s, v13.4s
	WORD $0x4ea887ff // add     v31.4s, v31.4s, v8.4s
	WORD $0x4eac877b // add     v27.4s, v27.4s, v12.4s
	WORD $0x0f158f97 // rshrn   v23.4h, v28.4s, #11
	WORD $0x0f158fb5 // rshrn   v21.4h, v29.4s, #11
	WORD $0x0f158fd3 // rshrn   v19.4h, v30.4s, #11
	WORD $0x0f158ff1 // rshrn   v17.4h, v31.4s, #11
	WORD $0x4f158f17 // rshrn2  v23.8h, v24.4s, #11
	WORD $0x4f158f35 // rshrn2  v21.8h, v25.4s, #11
	WORD $0x4f158f53 // rshrn2  v19.8h, v26.4s, #11
	WORD $0x4f158f71 // rshrn2  v17.8h, v27.4s, #11
	WORD $0x4e512a1f // trn1    v31.8h, v16.8h, v17.8h
	WORD $0x4e532a42 // trn1    v2.8h, v18.8h, v19.8h
	WORD $0x4e552a83 // trn1    v3.8h, v20.8h, v21.8h
	WORD $0x4e572ac4 // trn1    v4.8h, v22.8h, v23.8h
	WORD $0x4e516a11 // trn2    v17.8h, v16.8h, v17.8h
	WORD $0x4e536a53 // trn2    v19.8h, v18.8h, v19.8h
	WORD $0x4e556a95 // trn2    v21.8h, v20.8h, v21.8h
	WORD $0x4e576ad7 // trn2    v23.8h, v22.8h, v23.8h
	WORD $0x4e842874 // trn1    v20.4s, v3.4s, v4.4s
	WORD $0x4e846864 // trn2    v4.4s, v3.4s, v4.4s
	WORD $0x4e822be3 // trn1    v3.4s, v31.4s, v2.4s
	WORD $0x4e826bf2 // trn2    v18.4s, v31.4s, v2.4s
	WORD $0x4e932a3f // trn1    v31.4s, v17.4s, v19.4s
	WORD $0x4e936a33 // trn2    v19.4s, v17.4s, v19.4s
	WORD $0x4e976aa2 // trn2    v2.4s, v21.4s, v23.4s
	WORD $0x4e972ab5 // trn1    v21.4s, v21.4s, v23.4s
	WORD $0x4ec46a56 // trn2    v22.2d, v18.2d, v4.2d
	WORD $0x4ed42870 // trn1    v16.2d, v3.2d, v20.2d
	WORD $0x4ed52bf1 // trn1    v17.2d, v31.2d, v21.2d
	WORD $0x4ec26a77 // trn2    v23.2d, v19.2d, v2.2d
	WORD $0x4ec42a52 // trn1    v18.2d, v18.2d, v4.2d
	WORD $0x4ed46874 // trn2    v20.2d, v3.2d, v20.2d
	WORD $0x4ec22a73 // trn1    v19.2d, v19.2d, v2.2d
	WORD $0x4ed56bf5 // trn2    v21.2d, v31.2d, v21.2d
	WORD $0x4e778618 // add     v24.8h, v16.8h, v23.8h
	WORD $0x6e77861f // sub     v31.8h, v16.8h, v23.8h
	WORD $0x4e768639 // add     v25.8h, v17.8h, v22.8h
	WORD $0x6e76863e // sub     v30.8h, v17.8h, v22.8h
	WORD $0x4e75865a // add     v26.8h, v18.8h, v21.8h
	WORD $0x6e75865d // sub     v29.8h, v18.8h, v21.8h
	WORD $0x4e74867b // add     v27.8h, v19.8h, v20.8h
	WORD $0x6e74867c // sub     v28.8h, v19.8h, v20.8h
	WORD $0x4e7b8708 // add     v8.8h, v24.8h, v27.8h
	WORD $0x6e7b8709 // sub     v9.8h, v24.8h, v27.8h
	WORD $0x4e7a872a // add     v10.8h, v25.8h, v26.8h
	WORD $0x6e7a872b // sub     v11.8h, v25.8h, v26.8h
	WORD $0x4e6a8510 // add     v16.8h, v8.8h, v10.8h
	WORD $0x6e6a8514 // sub     v20.8h, v8.8h, v10.8h
	WORD $0x4e698572 // add     v18.8h, v11.8h, v9.8h
	WORD $0x4f1e2610 // srshr   v16.8h, v16.8h, #2
	WORD $0x4f1e2694 // srshr   v20.8h, v20.8h, #2
	WORD $0x4f60a258 // smull2  v24.4s, v18.8h, v0.h[2]
	WORD $0x0f60a252 // smull   v18.4s, v18.4h, v0.h[2]
	WORD $0x4eb21e56 // mov     v22.16b, v18.16b
	WORD $0x4eb81f19 // mov     v25.16b, v24.16b
	WORD $0x0f702132 // smlal   v18.4s, v9.4h, v0.h[3]
	WORD $0x4f702138 // smlal2  v24.4s, v9.8h, v0.h[3]
	WORD $0x0f702976 // smlal   v22.4s, v11.4h, v0.h[7]
	WORD $0x4f702979 // smlal2  v25.4s, v11.8h, v0.h[7]
	WORD $0x0f118e52 // rshrn   v18.4h, v18.4s, #15
	WORD $0x0f118ed6 // rshrn   v22.4h, v22.4s, #15
	WORD $0x4f118f12 // rshrn2  v18.8h, v24.4s, #15
	WORD $0x4f118f36 // rshrn2  v22.8h, v25.4s, #15
	WORD $0x4e7f8788 // add     v8.8h, v28.8h, v31.8h
	WORD $0x4e7e87a9 // add     v9.8h, v29.8h, v30.8h
	WORD $0x4e7e878a // add     v10.8h, v28.8h, v30.8h
	WORD $0x4e7f87ab // add     v11.8h, v29.8h, v31.8h
	WORD $0x0f50a944 // smull   v4.4s, v10.4h, v0.h[5]
	WORD $0x4f50a945 // smull2  v5.4s, v10.8h, v0.h[5]
	WORD $0x0f502964 // smlal   v4.4s, v11.4h, v0.h[5]
	WORD $0x4f502965 // smlal2  v5.4s, v11.8h, v0.h[5]
	WORD $0x4f40a398 // smull2  v24.4s, v28.8h, v0.h[0]
	WORD $0x4f51a3b9 // smull2  v25.4s, v29.8h, v1.h[1]
	WORD $0x4f71a3da // smull2  v26.4s, v30.8h, v1.h[3]
	WORD $0x4f60abfb // smull2  v27.4s, v31.8h, v0.h[6]
	WORD $0x0f40a39c // smull   v28.4s, v28.4h, v0.h[0]
	WORD $0x0f51a3bd // smull   v29.4s, v29.4h, v1.h[1]
	WORD $0x0f71a3de // smull   v30.4s, v30.4h, v1.h[3]
	WORD $0x0f60abff // smull   v31.4s, v31.4h, v0.h[6]
	WORD $0x4f40a90c // smull2  v12.4s, v8.8h, v0.h[4]
	WORD $0x4f61a12d // smull2  v13.4s, v9.8h, v1.h[2]
	WORD $0x4f41a14e // smull2  v14.4s, v10.8h, v1.h[0]
	WORD $0x4f50a16f // smull2  v15.4s, v11.8h, v0.h[1]
	WORD $0x0f40a908 // smull   v8.4s, v8.4h, v0.h[4]
	WORD $0x0f61a129 // smull   v9.4s, v9.4h, v1.h[2]
	WORD $0x0f41a14a // smull   v10.4s, v10.4h, v1.h[0]
	WORD $0x0f50a16b // smull   v11.4s, v11.4h, v0.h[1]
	WORD $0x4ea4854a // add     v10.4s, v10.4s, v4.4s
	WORD $0x4ea585ce // add     v14.4s, v14.4s, v5.4s
	WORD $0x4ea4856b // add     v11.4s, v11.4s, v4.4s
	WORD $0x4ea585ef // add     v15.4s, v15.4s, v5.4s
	WORD $0x4ea8879c // add     v28.4s, v28.4s, v8.4s
	WORD $0x4eac8718 // add     v24.4s, v24.4s, v12.4s
	WORD $0x4ea987bd // add     v29.4s, v29.4s, v9.4s
	WORD $0x4ead8739 // add     v25.4s, v25.4s, v13.4s
	WORD $0x4eaa87de // add     v30.4s, v30.4s, v10.4s
	WORD $0x4eae875a // add     v26.4s, v26.4s, v14.4s
	WORD $0x4eab87ff // add     v31.4s, v31.4s, v11.4s
	WORD $0x4eaf877b // add     v27.4s, v27.4s, v15.4s
	WORD $0x4eaa879c // add     v28.4s, v28.4s, v10.4s
	WORD $0x4eae8718 // add     v24.4s, v24.4s, v14.4s
	WORD $0x4eab87bd // add     v29.4s, v29.4s, v11.4s
	WORD $0x4eaf8739 // add     v25.4s, v25.4s, v15.4s
	WORD $0x4ea987de // add     v30.4s, v30.4s, v9.4s
	WORD $0x4ead875a // add     v26.4s, v26.4s, v13.4s
	WORD $0x4ea887ff // add     v31.4s, v31.4s, v8.4s
	WORD $0x4eac877b // add     v27.4s, v27.4s, v12.4s
	WORD $0x0f118f97 // rshrn   v23.4h, v28.4s, #15
	WORD $0x0f118fb5 // rshrn   v21.4h, v29.4s, #15
	WORD $0x0f118fd3 // rshrn   v19.4h, v30.4s, #15
	WORD $0x0f118ff1 // rshrn   v17.4h, v31.4s, #15
	WORD $0x4f118f17 // rshrn2  v23.8h, v24.4s, #15
	WORD $0x4f118f35 // rshrn2  v21.8h, v25.4s, #15
	WORD $0x4f118f53 // rshrn2  v19.8h, v26.4s, #15
	WORD $0x4f118f71 // rshrn2  v17.8h, v27.4s, #15
	WORD $0x4c9f2410 // st1     {v16.8h-v19.8h}, [x0], #64
	WORD $0x4c002414 // st1     {v20.8h-v23.8h}, [x0]
	WORD $0x0cdf23e8 // ld1     {v8.8b-v11.8b}, [sp], #32
	WORD $0x0cdf23ec // ld1     {v12.8b-v15.8b}, [sp], #32
	RET              // br      x30

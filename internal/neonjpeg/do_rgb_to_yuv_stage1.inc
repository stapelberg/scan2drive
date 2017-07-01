	// gas: ushll           v4.8h, v10.8b, #0  /* r = v4 */
	// obj: uxtl    v4.8h, v10.8b
	WORD $0x2f08a544
	// gas: ushll           v6.8h, v11.8b, #0  /* g = v6 */
	// obj: uxtl    v6.8h, v11.8b
	WORD $0x2f08a566
	// gas: ushll           v8.8h, v12.8b, #0  /* b = v8 */
	// obj: uxtl    v8.8h, v12.8b
	WORD $0x2f08a588
	// gas: rev64           v18.4s, v1.4s
	WORD $0x4ea00832
	// gas: rev64           v26.4s, v1.4s
        WORD $0x4ea0083a
	// gas: rev64           v28.4s, v1.4s
        WORD $0x4ea0083c
	// gas: rev64           v30.4s, v1.4s
        WORD $0x4ea0083e
	// gas: umull           v14.4s, v4.4h, v0.h[0]
        WORD $0x2f40a08e
	// gas: umull2          v16.4s, v4.8h, v0.h[0]
        WORD $0x6f40a090
	// gas: umlsl           v18.4s, v4.4h, v0.h[3]
        WORD $0x2f706092
	// gas: umlsl2          v26.4s, v4.8h, v0.h[3]
        WORD $0x6f70609a
	// gas: umlal           v28.4s, v4.4h, v0.h[5]
        WORD $0x2f50289c
	// gas: umlal2          v30.4s, v4.8h, v0.h[5]
        WORD $0x6f50289e
	// gas: umlal           v14.4s, v6.4h, v0.h[1]
        WORD $0x2f5020ce
	// gas: umlal2          v16.4s, v6.8h, v0.h[1]
        WORD $0x6f5020d0
	// gas: umlsl           v18.4s, v6.4h, v0.h[4]
        WORD $0x2f4068d2
	// gas: umlsl2          v26.4s, v6.8h, v0.h[4]
        WORD $0x6f4068da
	// gas: umlsl           v28.4s, v6.4h, v0.h[6]
        WORD $0x2f6068dc
	// gas: umlsl2          v30.4s, v6.8h, v0.h[6]
        WORD $0x6f6068de
	// gas: umlal           v14.4s, v8.4h, v0.h[2]
        WORD $0x2f60210e
	// gas: umlal2          v16.4s, v8.8h, v0.h[2]
        WORD $0x6f602110
	// gas: umlal           v18.4s, v8.4h, v0.h[5]
        WORD $0x2f502912
	// gas: umlal2          v26.4s, v8.8h, v0.h[5]
        WORD $0x6f50291a
	// gas: umlsl           v28.4s, v8.4h, v0.h[7]
        WORD $0x2f70691c
	// gas: umlsl2          v30.4s, v8.8h, v0.h[7]
        WORD $0x6f70691e

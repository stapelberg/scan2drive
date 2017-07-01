	// gas: ld3 {v10.8b, v11.8b, v12.8b}, [RGB], #24
	// obj: ld3 {v10.8b-v12.8b}, [x7], #24
	WORD $0x0cdf40ea
	// gas: prfm pldl1keep, [RGB, #128]
	// obj: prfm pldl1keep, [x7, #128]
	WORD $0xf98040e0

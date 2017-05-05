// Copyright 2016 Michael Stapelberg and contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

#include "textflag.h"

// func rgb2rgba(res []byte, pixels []byte)
TEXT ·rgb2rgba(SB),NOSPLIT,$0
	MOVD res+0(FP), R0 // can now use 0(R0) to address res[0]
	MOVD pixels+24(FP), R1
	MOVD pixels_len+32(FP), R2

	// R3 points to the last pixel element.
	ADD R2, R1, R3

	// The A value is always 0xFF
	WORD $0x0F07E7E3 // movi v3.8b, #0xFF

loop:
	// fill the 8-byte vectors v0, v1, v2 with:
	// pixels[0], pixels[3], …, pixels[21]
	// pixels[1], pixels[4], …, pixels[22]
	// pixels[2], pixels[5], …, pixels[23]
	// then increment R1 by 24 bytes
	WORD $0x0CDF4020 // ld3	{v0.8b-v2.8b}, [x1], #24

	// store the 8-byte vectors v0, v1, v2, v3 to:
	// res[0], res[4], …, res[28]
	// res[1], res[5], …, res[29]
	// res[2], res[6], …, res[30]
	// res[3], res[7], …, res[31]
	// then increment R0 by 32 bytes
	WORD $0x0C9F0000 // st4 {v0.8b-v3.8b}, [x0], #32

	// If R1 is not pointing to the last element,
	// we got more bytes to convert.
	CMP R1, R3
	BNE loop

	RET

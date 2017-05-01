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

package g3

type code struct {
	Length int // 0 <= Length <= 13
	Value  int // 0 <= Value <= 0xFF
}

var endOfLine = code{12, 0x1}

// The following tables were copied from tables 2/T.4, 3a/T.4 and
// 3b/T.4 from ITU-T T.4 (07/2003), section 4.1:
// https://www.itu.int/rec/T-REC-T.4-200307-I/en

// They have been programmatically checked to contain the same values
// as Sun’s java implementation:
// https://github.com/dcm4che/jai-imageio-core/blob/2cf790505c5f7512ea8afafa7bd53ce305438a80/src/main/java/com/sun/media/imageioimpl/plugins/tiff/TIFFFaxCompressor.java#L89-L173

// Terminating codes as per Table 2/T.4 from ITU-T T.4 (07/2003).
var terminatingCodesWhite = [64]code{
	{8, 0x35}, // 00110101b, run length  0
	{6, 0x7},  //   000111b, run length  1
	{4, 0x7},  //     0111b, run length  2
	{4, 0x8},  //     1000b, run length  3
	{4, 0xb},  //     1011b, run length  4
	{4, 0xc},  //     1100b, run length  5
	{4, 0xe},  //     1110b, run length  6
	{4, 0xf},  //     1111b, run length  7
	{5, 0x13}, //    10011b, run length  8
	{5, 0x14}, //    10100b, run length  9

	{5, 0x7},  //    00111b, run length 10
	{5, 0x8},  //    01000b, run length 11
	{6, 0x8},  //   001000b, run length 12
	{6, 0x3},  //   001000b, run length 13
	{6, 0x34}, //   001000b, run length 14
	{6, 0x35}, //   001000b, run length 15
	{6, 0x2a}, //   001000b, run length 16
	{6, 0x2b}, //   001000b, run length 17
	{7, 0x27}, //  0100111b, run length 18
	{7, 0xc},  //  0001100b, run length 19

	{7, 0x8},  //  0001000b, run length 20
	{7, 0x17}, //  0010111b, run length 21
	{7, 0x3},  //  0000011b, run length 22
	{7, 0x4},  //  0000100b, run length 23
	{7, 0x28}, //  0101000b, run length 24
	{7, 0x2b}, //  0101011b, run length 25
	{7, 0x13}, //  0010011b, run length 26
	{7, 0x24}, //  0100100b, run length 27
	{7, 0x18}, //  0011000b, run length 28
	{8, 0x2},  // 00000010b, run length 29

	{8, 0x3},  // 00000011b, run length 30
	{8, 0x1a}, // 00011010b, run length 31
	{8, 0x1b}, // 00011011b, run length 32
	{8, 0x12}, // 00010010b, run length 33
	{8, 0x13}, // 00010011b, run length 34
	{8, 0x14}, // 00010100b, run length 35
	{8, 0x15}, // 00010101b, run length 36
	{8, 0x16}, // 00010110b, run length 37
	{8, 0x17}, // 00010111b, run length 38
	{8, 0x28}, // 00101000b, run length 39

	{8, 0x29}, // 00101001b, run length 40
	{8, 0x2a}, // 00101010b, run length 41
	{8, 0x2b}, // 00101011b, run length 42
	{8, 0x2c}, // 00101100b, run length 43
	{8, 0x2d}, // 00101101b, run length 44
	{8, 0x4},  // 00000100b, run length 45
	{8, 0x5},  // 00000101b, run length 46
	{8, 0xa},  // 00001010b, run length 47
	{8, 0xb},  // 00001011b, run length 48
	{8, 0x52}, // 01010010b, run length 49

	{8, 0x53}, // 01010011b, run length 50
	{8, 0x54}, // 01010100b, run length 51
	{8, 0x55}, // 01010101b, run length 52
	{8, 0x24}, // 00100100b, run length 53
	{8, 0x25}, // 00100101b, run length 54
	{8, 0x58}, // 01011000b, run length 55
	{8, 0x59}, // 01011001b, run length 56
	{8, 0x5a}, // 01011010b, run length 57
	{8, 0x5b}, // 01011011b, run length 58
	{8, 0x4a}, // 01001010b, run length 59

	{8, 0x4b}, // 01001011b, run length 60
	{8, 0x32}, // 00110010b, run length 61
	{8, 0x33}, // 00110011b, run length 62
	{8, 0x34}, // 00110100b, run length 63
}

// Terminating codes as per Table 2/T.4 from ITU-T T.4 (07/2003).
var terminatingCodesBlack = [64]code{
	{10, 0x37}, //   0000110111b, run length  0
	{3, 0x2},   //          010b, run length  1
	{2, 0x3},   //           11b, run length  2
	{2, 0x2},   //           10b, run length  3
	{3, 0x3},   //          011b, run length  4
	{4, 0x3},   //         0011b, run length  5
	{4, 0x2},   //         0010b, run length  6
	{5, 0x3},   //        00011b, run length  7
	{6, 0x5},   //       000101b, run length  8
	{6, 0x4},   //       000100b, run length  9

	{7, 0x4},   //      0000100b, run length 10
	{7, 0x5},   //      0000101b, run length 11
	{7, 0x7},   //      0000111b, run length 12
	{8, 0x4},   //     00000100b, run length 13
	{8, 0x7},   //     00000111b, run length 14
	{9, 0x18},  //    000011000b, run length 15
	{10, 0x17}, //   0000010111b, run length 16
	{10, 0x18}, //   0000011000b, run length 17
	{10, 0x8},  //   0000001000b, run length 18
	{11, 0x67}, //  00001100111b, run length 19

	{11, 0x68}, //  00001101000b, run length 20
	{11, 0x6c}, //  00001101100b, run length 21
	{11, 0x37}, //  00000110111b, run length 22
	{11, 0x28}, //  00000101000b, run length 23
	{11, 0x17}, //  00000010111b, run length 24
	{11, 0x18}, //  00000011000b, run length 25
	{12, 0xca}, // 000011001010b, run length 26
	{12, 0xcb}, // 000011001011b, run length 27
	{12, 0xcc}, // 000011001100b, run length 28
	{12, 0xcd}, // 000011001101b, run length 29

	{12, 0x68}, // 000001101000b, run length 30
	{12, 0x69}, // 000001101001b, run length 31
	{12, 0x6a}, // 000001101010b, run length 32
	{12, 0x6b}, // 000001101011b, run length 33
	{12, 0xd2}, // 000011010010b, run length 34
	{12, 0xd3}, // 000011010011b, run length 35
	{12, 0xd4}, // 000011010100b, run length 36
	{12, 0xd5}, // 000011010101b, run length 37
	{12, 0xd6}, // 000011010110b, run length 38
	{12, 0xd7}, // 000011010111b, run length 39

	{12, 0x6c}, // 000001101100b, run length 40
	{12, 0x6d}, // 000001101101b, run length 41
	{12, 0xda}, // 000011011010b, run length 42
	{12, 0xdb}, // 000011011011b, run length 43
	{12, 0x54}, // 000001010100b, run length 44
	{12, 0x55}, // 000001010101b, run length 45
	{12, 0x56}, // 000001010110b, run length 46
	{12, 0x57}, // 000001010111b, run length 47
	{12, 0x64}, // 000001100100b, run length 48
	{12, 0x65}, // 000001100101b, run length 49

	{12, 0x52}, // 000001010010b, run length 50
	{12, 0x53}, // 000001010011b, run length 51
	{12, 0x24}, // 000000100100b, run length 52
	{12, 0x37}, // 000000110111b, run length 53
	{12, 0x38}, // 000000111000b, run length 54
	{12, 0x27}, // 000000100111b, run length 55
	{12, 0x28}, // 000000101000b, run length 56
	{12, 0x58}, // 000001011000b, run length 57
	{12, 0x59}, // 000001011001b, run length 58
	{12, 0x2b}, // 000000101011b, run length 59

	{12, 0x2c}, // 000000101100b, run length 60
	{12, 0x5a}, // 000001011010b, run length 61
	{12, 0x66}, // 000001100110b, run length 62
	{12, 0x67}, // 000001100111b, run length 63
}

// Make-up codes as per Table 3a/T.4 from ITU-T T.4 (07/2003).
var makeupCodesWhite = [28 + 13]code{
	{0, 0},    //  write nothing, run length 0
	{5, 0x1b}, //     11011b, run length  1 * 64 =   64
	{5, 0x12}, //     10010b, run length  2 * 64 =  128
	{6, 0x17}, //    010111b, run length  3 * 64 =  192
	{7, 0x37}, //   0110111b, run length  4 * 64 =  256
	{8, 0x36}, //  00110110b, run length  5 * 64 =  320
	{8, 0x37}, //  00110111b, run length  6 * 64 =  384
	{8, 0x64}, //  01100100b, run length  7 * 64 =  448
	{8, 0x65}, //  01100101b, run length  8 * 64 =  512
	{8, 0x68}, //  01101000b, run length  9 * 64 =  576
	{8, 0x67}, //  01100111b, run length 10 * 64 =  640
	{9, 0xcc}, // 011001100b, run length 11 * 64 =  704
	{9, 0xcd}, // 011001101b, run length 12 * 64 =  768
	{9, 0xd2}, // 011010010b, run length 13 * 64 =  832
	{9, 0xd3}, // 011010011b, run length 14 * 64 =  896
	{9, 0xd4}, // 011010100b, run length 15 * 64 =  960
	{9, 0xd5}, // 011010101b, run length 16 * 64 = 1024
	{9, 0xd6}, // 011010110b, run length 17 * 64 = 1088
	{9, 0xd7}, // 011010111b, run length 18 * 64 = 1152
	{9, 0xd8}, // 011011000b, run length 19 * 64 = 1216
	{9, 0xd9}, // 011011001b, run length 20 * 64 = 1280
	{9, 0xda}, // 011011010b, run length 21 * 64 = 1344
	{9, 0xdb}, // 011011011b, run length 22 * 64 = 1408
	{9, 0x98}, // 010011000b, run length 23 * 64 = 1472
	{9, 0x99}, // 010011001b, run length 24 * 64 = 1536
	{9, 0x9a}, // 010011010b, run length 25 * 64 = 1600
	{6, 0x18}, //    011000b, run length 26 * 64 = 1664
	{9, 0x9b}, // 010011011b, run length 27 * 64 = 1728

	// Make-up codes as per Table 3b/T.4 from ITU-T T.4 (07/2003).
	{11, 0x8},  //  00000001000b, run length 28 * 64 = 1792
	{11, 0xc},  //  00000001100b, run length 29 * 64 = 1856
	{11, 0xd},  //  00000001101b, run length 30 * 64 = 1920
	{12, 0x12}, // 000000010010b, run length 31 * 64 = 1984
	{12, 0x13}, // 000000010011b, run length 32 * 64 = 2048
	{12, 0x14}, // 000000010100b, run length 33 * 64 = 2112
	{12, 0x15}, // 000000010101b, run length 34 * 64 = 2176
	{12, 0x16}, // 000000010110b, run length 35 * 64 = 2240
	{12, 0x17}, // 000000010111b, run length 36 * 64 = 2304
	{12, 0x1c}, // 000000011100b, run length 37 * 64 = 2368
	{12, 0x1d}, // 000000011101b, run length 38 * 64 = 2432
	{12, 0x1e}, // 000000011110b, run length 39 * 64 = 2496
	{12, 0x1f}, // 000000011111b, run length 40 * 64 = 2560
}

// Make-up codes as per Table 3a/T.4 from ITU-T T.4 (07/2003).
var makeupCodesBlack = [28 + 13]code{
	{0, 0},     //  write nothing, run length 0
	{10, 0xf},  //    0000001111b, run length  1 * 64 =  64
	{12, 0xc8}, //  000011001000b, run length  2 * 64 =  128
	{12, 0xc9}, //  000011001001b, run length  3 * 64 =  192
	{12, 0x5b}, //  000001011011b, run length  4 * 64 =  256
	{12, 0x33}, //  000000110011b, run length  5 * 64 =  320
	{12, 0x34}, //  000000110100b, run length  6 * 64 =  384
	{12, 0x35}, //  000000110101b, run length  7 * 64 =  448
	{13, 0x6c}, // 0000001101100b, run length  8 * 64 =  512
	{13, 0x6d}, // 0000001101101b, run length  9 * 64 =  576
	{13, 0x4a}, // 0000001001010b, run length 10 * 64 =  640
	{13, 0x4b}, // 0000001001011b, run length 11 * 64 =  704
	{13, 0x4c}, // 0000001001100b, run length 12 * 64 =  768
	{13, 0x4d}, // 0000001001101b, run length 13 * 64 =  832
	{13, 0x72}, // 0000001110010b, run length 14 * 64 =  896
	{13, 0x73}, // 0000001110011b, run length 15 * 64 =  960
	{13, 0x74}, // 0000001110100b, run length 16 * 64 = 1024
	{13, 0x75}, // 0000001110101b, run length 17 * 64 = 1088
	{13, 0x76}, // 0000001110110b, run length 18 * 64 = 1152
	{13, 0x77}, // 0000001110111b, run length 19 * 64 = 1216
	{13, 0x52}, // 0000001010010b, run length 20 * 64 = 1280
	{13, 0x53}, // 0000001010011b, run length 21 * 64 = 1344
	{13, 0x54}, // 0000001010100b, run length 22 * 64 = 1408
	{13, 0x55}, // 0000001010101b, run length 23 * 64 = 1472
	{13, 0x5a}, // 0000001011010b, run length 24 * 64 = 1536
	{13, 0x5b}, // 0000001011011b, run length 25 * 64 = 1600
	{13, 0x64}, // 0000001100100b, run length 26 * 64 = 1664
	{13, 0x65}, // 0000001100101b, run length 27 * 64 = 1728

	// Make-up codes as per Table 3b/T.4 from ITU-T T.4 (07/2003).
	{11, 0x8},  //  00000001000b, run length 28 * 64 = 1792
	{11, 0xc},  //  00000001100b, run length 29 * 64 = 1856
	{11, 0xd},  //  00000001101b, run length 30 * 64 = 1920
	{12, 0x12}, // 000000010010b, run length 31 * 64 = 1984
	{12, 0x13}, // 000000010011b, run length 32 * 64 = 2048
	{12, 0x14}, // 000000010100b, run length 33 * 64 = 2112
	{12, 0x15}, // 000000010101b, run length 34 * 64 = 2176
	{12, 0x16}, // 000000010110b, run length 35 * 64 = 2240
	{12, 0x17}, // 000000010111b, run length 36 * 64 = 2304
	{12, 0x1c}, // 000000011100b, run length 37 * 64 = 2368
	{12, 0x1d}, // 000000011101b, run length 38 * 64 = 2432
	{12, 0x1e}, // 000000011110b, run length 39 * 64 = 2496
	{12, 0x1f}, // 000000011111b, run length 40 * 64 = 2560
}

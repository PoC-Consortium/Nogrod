// (c) 2018 PoC Consortium ALL RIGHTS RESERVED

#include <stdint.h>
#include <stdbool.h>

uint32_t
calculate_scoop(uint64_t height, uint8_t *gensig);

void
calculate_deadlines_sse4(
          uint32_t scoop,  uint64_t base_target, uint8_t* gensig, bool poc2,

          uint64_t addr1,  uint64_t addr2,  uint64_t addr3,  uint64_t addr4,

          uint64_t nonce1, uint64_t nonce2, uint64_t nonce3, uint64_t nonce4,

          uint64_t* deadline1,uint64_t* deadline2,uint64_t* deadline3,uint64_t* deadline4);

void
calculate_deadlines_avx2(
          uint32_t scoop,  uint64_t base_target, uint8_t* gensig, bool poc2,

          uint64_t addr1,  uint64_t addr2,  uint64_t addr3,  uint64_t addr4,
          uint64_t addr5,  uint64_t addr6,  uint64_t addr7,  uint64_t addr8,

          uint64_t nonce1, uint64_t nonce2, uint64_t nonce3, uint64_t nonce4,
          uint64_t nonce5, uint64_t nonce6, uint64_t nonce7, uint64_t nonce8,

          uint64_t* deadline1,uint64_t* deadline2,uint64_t* deadline3,uint64_t* deadline4,
          uint64_t* deadline5,uint64_t* deadline6,uint64_t* deadline7,uint64_t* deadline8);

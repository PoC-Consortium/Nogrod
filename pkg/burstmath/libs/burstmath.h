#include <stdbool.h>
#include <stdint.h>

typedef struct {
  uint64_t account_id;
  uint64_t nonce;
  uint32_t scoop_nr;
  uint64_t base_target;
  uint8_t *gen_sig;
  uint64_t deadline;
} CalcDeadlineRequest;

uint32_t calculate_scoop(uint64_t height, uint8_t *gensig);

void calculate_deadline(CalcDeadlineRequest *req);

void calculate_deadlines_sse4(CalcDeadlineRequest **reqs);

void calculate_deadlines_avx2(CalcDeadlineRequest **reqs);

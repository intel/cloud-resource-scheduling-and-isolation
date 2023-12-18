//go:build ignore

#include "headers/common.h"

char __license[] SEC("license") = "Dual MIT/GPL";



// Max number of disks we expect to see on the host
const u8 max_disks = 20;

// 16 buckets per disk in kib, max range is 128k
const u8 max_size_slot = 9;
const u8 max_cgroup = 200;


// This struct is defined according to the following format file:
// /sys/kernel/tracing/events/block/block_bio_queue/format
struct block_rq {
    /* The first 8 bytes is not allowed to read */
    unsigned long pad;

    unsigned int dev;  // major:minor
    unsigned long sector;
    unsigned int nr_sector; // number of sectors
    char rwbs[8]; // operation
    char comm[16];
};

struct filter_key {
    unsigned long cgroup;
};

struct block_io_key{
    unsigned long cgroup;  // 100 - container  200 only cared pod
    unsigned int operation; // 2
    unsigned int slot; // 0-8   1k, 2k, 4k, 8k, 16k, 32k, 64k, 128k, 128k+
    unsigned int dev; //
};

struct bpf_map_def SEC("maps") counting_map = {
    .type        = BPF_MAP_TYPE_HASH,
    .key_size    = sizeof(struct block_io_key),
    .value_size  = sizeof(u64),
    .max_entries = (max_size_slot + 2) * max_disks*max_cgroup,
};

struct bpf_map_def SEC("maps") filter_map = {
    .type        = BPF_MAP_TYPE_HASH,
    .key_size    = sizeof(struct filter_key),
    .value_size  = sizeof(u8),
    .max_entries = max_cgroup,
};

SEC("tracepoint/block/block_bio_queue")
int block_bio_queue(struct block_rq *rq) {
    u64 initval = 1;
    struct block_io_key key = {};

    // add cgroup id
    key.cgroup = bpf_get_current_cgroup_id();
    u8 *val = bpf_map_lookup_elem(&filter_map, &key.cgroup);
    if (!val) {
        return 0;
    }

    u64 size_slot = 0,size=rq-> nr_sector;
    size >>= 1;
    while (size >0){
        size_slot++;
        size >>= 1;
    }
    if (size_slot > max_size_slot)
    {
        size_slot = max_size_slot;
    }
    key.slot = size_slot;
    bpf_probe_read(&key.dev, sizeof(key.dev), &rq->dev);


    // add operation
    key.operation = rq->rwbs[0] == 'R' ? 0 : 1;

    u64 *valp = bpf_map_lookup_elem(&counting_map, &key);
    if (!valp) {
        bpf_map_update_elem(&counting_map, &key, &initval, BPF_ANY);
    } else {
        __sync_fetch_and_add(valp, 1);
    }
    return 0;
}

### **Intro (topic summary)**  
We’ve built a new virtual backend for **rclone** called **cRaid3**, combining three remotes into one fault‑tolerant storage system. It’s an early implementation, and we’d love your feedback, tests, and design input!

***

## Solving the Failing Remote Problem — New Virtual Backend: **cRaid3** (Request for Comments)

Dear rclone community,

Hard disks fail. That's why we have RAID — multiple drives working together so that when one goes down, your data stays safe and accessible.  
The same principle applies to cloud storage: an **account can get compromised**, a **provider can disappear**, or access to a **geographic region**, or even to entire organizations like **NGOs** or **companies**, can suddenly be blocked. When that happens, both current and historical data may be at risk.

To address this, we built **cloud raid3**,  **cRaid3** or in short **raid3** — a new **virtual backend for rclone** that combines **three remotes into one fault‑tolerant storage system**.

***

### How it works

Imagine you have storage providers in the **US**, **New Zealand**, and **France**.  
You bundle them into a single virtual remote called `safestorage` and use it like any other remote:

```bash
$ rclone ls safestorage:
```

If the New Zealand provider fails, **all your data remains fully accessible for reading**.  
`safestorage` reports which backend is missing, and rebuilding uses only the data stored on the two working systems.  
You can then set up a new provider in Australia, update your `rclone.conf`, and rebuild:

```bash
$ rclone backend rebuild safestorage:
```

That’s it — `safestorage` is ready for storing data again and your data is **fault‑tolerant** again.

***

### Technical details

RAID3 splits data at the **byte level** across three backends:

- Even‑indexed bytes → *even* remote  
- Odd‑indexed bytes → *odd* remote  
- XOR parity of each byte pair → *parity* remote  

If one backend fails, the missing data is reconstructed from the other two:

- Missing even → computed from odd XOR parity  
- Missing odd → computed from even XOR parity  
- Missing parity → recalculated from even XOR odd  

This provides **fault tolerance with only ~50 % storage overhead**.

***

### Demo available

Integration test scripts and a setup helper are included:

```bash
$ cd backend/raid3/integration && ./setup.sh
$ rclone --config $(cat ${HOME}/.rclone_raid3_integration_tests.workdir)/rclone_raid3_integration_tests.config ls localraid3:
```

Make sure to `go build` and `go install` the forked rclone binary before testing.  
If you have **MinIO** running in Docker, the provided config also includes a `minioraid3` backend.

***

### 💬 Request for feedback

This is an **alpha/experimental** implementation — core functionality is working and tested.  
**Current status:**
- ✅ Streaming support implemented (default: `use_streaming=true`)
- ✅ Degraded mode reads (works with 2/3 backends, automatic reconstruction)
- ✅ Automatic heal (`auto_heal=true` by default, background particle restoration)
- ✅ Auto-cleanup (`auto_cleanup=true` by default, removes orphaned objects)
- ✅ Backend commands: `status`, `rebuild`, `heal`
- ✅ Test suite: 96 PASS, 0 FAIL (fs/sync and fs/operations tests)

We'd appreciate feedback from the community, especially on issues such as:

- Should we call our new backend **cRaid3** or simply **raid3** ?  
- What should `rclone size` return — original data size or total across all parts?  
- How should `rclone md5sum` behave — should we store the original file's checksum explicitly?  
- Could the **chunker** or **crypt** virtual remote wrap the **raid3** remote?

**Known limitations:**
Update operation rollback has issues when `rollback=true` (Put and Move rollback work correctly), see [`docs/OPEN_QUESTIONS.md`](docs/OPEN_QUESTIONS.md) for details.

The implementation is available for download and testing here: **https://github.com/Breschling/rclone.git** . 

***

### Why RAID3?

RAID3 is **fast, simple, deterministic, and state‑light**.  
In traditional disk arrays, the parity disk was a bottleneck — but in **cloud storage this limitation doesn't exist**, making RAID3 an ideal starting point for reliable, multi‑provider redundancy.

***

### Future directions: more flexibility and encryption?

As we refine raid3, we hope to identify what’s needed for stable, high‑performance distributed backends in rclone.  
If the community finds this approach useful, we plan to explore more advanced (but probably more demanding) options such as **Erasure Coding** and **Threshold Encryption** (see the 2021 forum topic [*“Can we add erasure coding to rclone?”*](https://forum.rclone.org/t/can-we-add-erasure-coding-to-rclone/23684) between @hvrietsc (Hans) and @ncw (Nick)).

***

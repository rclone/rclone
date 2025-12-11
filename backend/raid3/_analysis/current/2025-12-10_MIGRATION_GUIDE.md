# Migration Guide: level3 → raid3

After renaming the backend from `level3` to `raid3`, you need to update your rclone configuration.

## Quick Fix (Minimal Changes)

Edit your rclone config file (`~/.config/rclone/rclone.conf`) and change:

```ini
[locallevel3]
type = level3    ← Change this line
even = ...
odd = ...
parity = ...
```

To:

```ini
[locallevel3]
type = raid3     ← Changed to raid3
even = ...
odd = ...
parity = ...
```

**That's it!** Your remote name can stay as `locallevel3` if you prefer.

---

## Full Migration (Recommended)

1. **Update the backend type:**
   ```ini
   [locallevel3]      ← Optionally rename this too
   type = raid3       ← Change from level3 to raid3
   even = ...
   odd = ...
   parity = ...
   ```

2. **Optional: Rename the remote section** (if you want to use new naming):
   ```ini
   [localraid3]       ← Renamed from locallevel3
   type = raid3
   even = ...
   odd = ...
   parity = ...
   ```

3. **If you renamed the remote**, update any scripts or automation:
   - Update scripts or automation that reference the old remote name
   - Or use `RAID3_REMOTE` environment variable to override the default name

---

## Verification

After updating your config, verify it works:

```bash
# Test the remote
rclone lsd locallevel3:    # or localraid3: if you renamed it

# Run the test script
./compare_raid3_with_single.sh test --storage-type=local -v
```

---

## Troubleshooting

**Error: "didn't find backend called 'level3'"**
→ Your config still has `type = level3`. Change it to `type = raid3`.

**Error: "didn't find section in config file ('localraid3')"**
→ Your config uses a different remote name. Either:
- Rename the section to `[localraid3]` in your config, OR
- Set `RAID3_REMOTE="yourremotename"` environment variable to match your config


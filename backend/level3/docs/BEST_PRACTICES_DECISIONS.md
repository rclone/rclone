# Best Practices for Documenting Design Decisions

**Date**: November 2, 2025  
**Purpose**: Guide for maintaining design decision documentation in the level3 project

---

## 🎯 Why Document Decisions?

**Benefits**:
1. **Future reference** - Remember why we chose this approach
2. **Onboarding** - Help new developers understand the project
3. **Avoid rework** - Don't reconsider already-decided questions
4. **Accountability** - Clear rationale for architectural choices
5. **Knowledge preservation** - Decisions outlive individual contributors

**Industry Standard**: Architecture Decision Records (ADRs) - used by major projects

---

## 📚 Documentation System for Level3

### File Structure:

```
backend/level3/
├── DESIGN_DECISIONS.md    ✅ Decided questions with rationale
├── OPEN_QUESTIONS.md      🤔 Questions awaiting decisions
├── README.md              📖 User documentation
├── RAID3.md               🔧 Technical specification
└── docs/                  📚 Detailed documentation
    ├── [Decision Name].md    (detailed analysis)
    └── README.md             (navigation guide)
```

---

## 🔄 Decision Workflow

### 1. Question Arises

**Capture in**: `OPEN_QUESTIONS.md`

```markdown
### Q#: Backend Help Command
**Question**: How should `rclone backend help level3:` behave?
**Options**: A) Aggregated, B) Per-remote, C) Custom
**Status**: 🔴 Open
```

**Purpose**: Don't lose the question!

---

### 2. Research & Analysis

**If complex**: Create detailed doc in `docs/`

Example: `docs/BACKEND_HELP_ANALYSIS.md`
- Research how union/combine work
- List pros/cons of each option
- Performance considerations
- User experience analysis

**If simple**: Just add notes to `OPEN_QUESTIONS.md`

---

### 3. Discussion & Decision

**Discuss with**: Team, maintainers, or yourself

**Consider**:
- User needs
- Rclone conventions
- Implementation complexity
- Performance impact
- Maintainability

---

### 4. Document Decision

**Move to**: `DESIGN_DECISIONS.md`

```markdown
### DD-009: Backend Help Command
**Date**: 2025-11-XX
**Status**: ✅ Accepted

**Decision**: Use Option C (level3-specific help)

**Rationale**: 
- More informative for users
- Shows RAID 3 specific behavior
- Doesn't require aggregating complex backend features

**Consequences**:
- ✅ Clear user experience
- ⚠️ Custom implementation needed
```

---

### 5. Implement

**Code changes** + **Tests** + **Update README**

---

### 6. Update OPEN_QUESTIONS.md

**Either**:
- Delete the question (if documented in DESIGN_DECISIONS.md)
- Or move to "Resolved" section with link

---

## 📝 Templates

### For Open Questions:

```markdown
### Q#: [CONCISE_TITLE]
**Date**: YYYY-MM-DD
**Status**: 🔴 Open | 🟡 Investigating | 🟢 Tentative

**Question**: [One clear sentence]

**Context**: [Why does this matter? What triggered this question?]

**Options**:
- **A**: [Approach 1]
  - Pros: [benefits]
  - Cons: [drawbacks]
- **B**: [Approach 2]
  - Pros: [benefits]
  - Cons: [drawbacks]

**Investigation Needed**:
- [ ] Research X
- [ ] Test Y
- [ ] Check how Z does it

**Recommendation**: [Initial thoughts]

**Priority**: High | Medium | Low

**References**: [Related docs/issues]
```

---

### For Design Decisions:

```markdown
### DD-XXX: [TITLE]
**Date**: YYYY-MM-DD  
**Status**: ✅ Accepted | ⚠️ Deprecated  

**Context**: [Problem statement]

**Decision**: [What we decided - be specific]

**Rationale**: [Why - the most important part!]

**Alternatives Considered**:
- **Option A**: [description] - [why rejected]
- **Option B**: [description] - [why rejected]

**Consequences**:
- ✅ Benefit 1
- ✅ Benefit 2
- ⚠️ Trade-off 1
- ❌ Limitation 1

**Implementation**: [Code changes needed/done]

**References**: [Links to detailed docs in docs/]
```

---

## 🎨 Best Practices

### 1. Write Decisions as History (Past Tense)

**Good**: "We decided to use strict writes because..."  
**Bad**: "We should use strict writes..."

**Reason**: Decisions are facts, not proposals

---

### 2. Explain the "Why", Not Just the "What"

**Good**: 
```
Decision: Use pre-flight health check
Rationale: Prevents rclone's retry logic from creating 
corrupted files when backends unavailable
```

**Bad**:
```
Decision: Use health check
```

**Reason**: Future maintainers need context!

---

### 3. List Alternatives Even if Obvious

**Why**: Shows you considered other options, not just picked first idea

**Example**:
```
Alternatives:
- Don't add health check (rejected - allows corruption)
- Use rollback instead (rejected - too complex)
- Trust errgroup (rejected - insufficient)
```

---

### 4. Update When Situation Changes

**Status flags**:
- ✅ Accepted - Currently implemented
- ⚠️ Deprecated - Was accepted, now replaced
- 🔴 Open - Not yet decided
- 🟡 Investigating - Research in progress
- 🟢 Tentative - Soft decision, may change

**Example**:
```
DD-002: Pre-flight Health Check
Status: ✅ Accepted (was: ⚠️ Deprecated)
Note: Initial implementation with retries was insufficient,
      replaced with health check approach
```

---

### 5. Link to Detailed Analysis

**In DESIGN_DECISIONS.md**: Short summary (1 paragraph)  
**In docs/**: Detailed analysis (multiple pages)

**Example**:
```
DD-001: Error Handling Policy
Decision: Strict writes
Rationale: [2-3 sentences]
References: docs/ERROR_HANDLING_ANALYSIS.md (5 pages)
            docs/ERROR_HANDLING_POLICY.md (official spec)
```

---

### 6. One Decision Per Section

**Good**: Each DD-XXX is a single decision  
**Bad**: Combining multiple related decisions

**Reason**: Easier to reference and update

---

### 7. Use Sequential Numbering

**DD-001, DD-002, DD-003...** (never reuse numbers)  
**Q1, Q2, Q3...** (can reuse when resolved)

**Reason**: Permanent references

---

## 🔍 When to Create Which Document

### OPEN_QUESTIONS.md
**When**:
- You have a question without a clear answer
- Multiple viable approaches exist
- Need to research/investigate
- Decision impacts users or architecture

**Format**: Question-focused, exploratory

---

### DESIGN_DECISIONS.md
**When**:
- Decision has been made
- Need to document rationale
- Want to avoid revisiting the decision
- Important for future maintenance

**Format**: Decision-focused, definitive

---

### docs/[TOPIC]_ANALYSIS.md
**When**:
- Question is complex (>1 page analysis)
- Need detailed research findings
- Multiple stakeholders need to review
- Want to preserve research for future

**Format**: Detailed analysis, comprehensive

---

### README.md / RAID3.md
**When**:
- Users need to know about the decision
- Affects how they use the backend
- Part of the specification

**Format**: User-friendly, concise

---

## 📊 Example Workflow

### Scenario: Your Question About Backend Help

**Step 1**: Add to OPEN_QUESTIONS.md ✅ (DONE)
```markdown
### Q1: Backend Help Command Behavior
**Question**: How should `rclone backend help level3:` behave?
**Options**: A) Aggregated, B) Per-remote, C) Custom
**Status**: 🔴 Open
```

**Step 2**: Investigate (when ready)
- Check union backend implementation
- Check combine backend implementation
- Create `docs/BACKEND_HELP_RESEARCH.md` if needed

**Step 3**: Discuss & Decide
- Consider user needs
- Choose best option (e.g., Option C)

**Step 4**: Document in DESIGN_DECISIONS.md
```markdown
### DD-009: Backend Help Command
**Decision**: Custom level3-specific help
**Rationale**: Most informative for RAID 3 users
```

**Step 5**: Implement
- Add backend help command
- Update README with example

**Step 6**: Clean up OPEN_QUESTIONS.md
- Remove Q1 or mark as resolved

---

## ✅ What We've Set Up for You

### Files Created:
1. **`DESIGN_DECISIONS.md`** - 8 decisions already documented
2. **`OPEN_QUESTIONS.md`** - 7 questions identified
3. **`docs/BEST_PRACTICES_DECISIONS.md`** (this file) - Guidelines

### Example Decisions Already Documented:
- DD-001: Hardware RAID 3 compliance
- DD-002: Pre-flight health check
- DD-003: Timeout modes
- DD-004: Self-healing with background workers
- DD-005: Parity filename suffixes
- DD-006: Byte-level striping
- DD-007: XOR parity for odd-length files
- DD-008: Test documentation structure

### Example Questions Already Captured:
- Q1: Backend help command (your example!)
- Q2: Streaming support
- Q3: Move with degraded source
- ... and 4 more

---

## 🎯 Recommendations

### For This Project:

**Use the lightweight approach**:
1. ✅ `DESIGN_DECISIONS.md` - Quick reference for decisions
2. ✅ `OPEN_QUESTIONS.md` - Track undecided items
3. ✅ `docs/` - Detailed analysis when needed

**Don't**:
- ❌ Don't create heavy process (keep it simple)
- ❌ Don't document trivial decisions
- ❌ Don't create docs that won't be maintained

**Do**:
- ✅ Document important decisions with rationale
- ✅ Capture questions when they arise
- ✅ Link to detailed analysis
- ✅ Update as situations evolve

---

## 📖 Industry Standards

### Architecture Decision Records (ADR)

**What it is**: Lightweight standard for documenting decisions

**Format**: Exactly what we're using!
- Context
- Decision
- Consequences
- Alternatives

**Used by**: Spotify, ThoughtWorks, GitHub, many open-source projects

**Our approach**: Simplified ADR (less formal, more practical)

---

## ✨ Summary

**Best Practice**: Use lightweight decision documentation

**For level3**:
- ✅ `DESIGN_DECISIONS.md` - Decided items
- ✅ `OPEN_QUESTIONS.md` - Open items
- ✅ `docs/` - Detailed analysis

**Process**:
1. Question arises → Add to OPEN_QUESTIONS.md
2. Research if needed → Create docs/[TOPIC]_ANALYSIS.md
3. Decision made → Document in DESIGN_DECISIONS.md
4. Implement → Code + tests + user docs

**Benefits**:
- ✅ No questions lost
- ✅ Rationale preserved
- ✅ Easy to maintain
- ✅ Helps future contributors

---

**Your question about `rclone backend help` is now tracked in OPEN_QUESTIONS.md as Q1!** 🎯


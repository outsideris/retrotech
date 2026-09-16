package research

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

type RuleSelection struct {
	JobID string `json:"jobId"`
	Rule  string `json:"rule"`
}

func (s *Store) ApplyRules(selections []RuleSelection) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.checkFiles(); e != nil {
		return 0, e
	}
	if len(selections) == 0 || len(selections) > 30 {
		return 0, errors.New("반영할 조사 규칙을 선택해 주세요")
	}
	// Separate reports may share this skill; serialize their read/append/write.
	lock, e := os.OpenFile(filepath.Join(s.SkillDir, ".follow-up.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return 0, e
	}
	defer lock.Close()
	if e = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); e != nil {
		return 0, e
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	skillPath := filepath.Join(s.SkillDir, "SKILL.md")
	skill, e := os.ReadFile(skillPath)
	if e != nil {
		return 0, e
	}
	learnPath := filepath.Join(s.SkillDir, "references/follow-up-learnings.md")
	learning, e := os.ReadFile(learnPath)
	if e != nil && !os.IsNotExist(e) {
		return 0, e
	}
	if len(learning) == 0 {
		learning = []byte("# Reviewed follow-up research lessons\n\nApply these host-reviewed questions when relevant to a new episode. They are research checks, not independently verified historical facts or a fixed episode outline.\n")
	}
	seen := map[string]bool{}
	chosen := []RuleSelection{}
	for _, v := range selections {
		v.Rule = strings.TrimSpace(v.Rule)
		j := s.job(v.JobID)
		if j == nil || j.Status != "completed" || j.Result == nil || v.Rule == "" || len([]rune(v.Rule)) > 1200 {
			return 0, errors.New("완료된 조사와 1~1,200자의 규칙을 선택해 주세요")
		}
		if seen[v.JobID] {
			return 0, errors.New("같은 조사 규칙을 두 번 선택할 수 없습니다")
		}
		seen[v.JobID] = true
		marker := "<!-- research-rule:" + hash([]byte(v.Rule))[:24] + " -->"
		if !strings.Contains(string(learning), marker) {
			learning = append(learning, []byte(fmt.Sprintf("\n%s\n\n## %s · %s\n\n%s\n\n- Originating research gap: %s\n", marker, now()[:10], strings.ReplaceAll(j.Result.Title, "\n", " "), v.Rule, strings.ReplaceAll(j.Result.Learning.Gap, "\n", " ")))...)
		}
		chosen = append(chosen, v)
	}
	if !strings.Contains(string(skill), "references/follow-up-learnings.md") {
		skill = append(skill, []byte("\n## Learn from reviewed follow-up questions\n\nFor new episodes and coverage reviews, read [host-reviewed research lessons](references/follow-up-learnings.md) and apply only the questions relevant to the current subject. Preserve their evidence requirements without importing another episode's historical claims.\n")...)
	}
	backup := filepath.Join(s.Dir, ".research/skill-backups", newID())
	if e = os.MkdirAll(backup, 0700); e != nil {
		return 0, e
	}
	oldSkill, _ := os.ReadFile(skillPath)
	oldLearn, _ := os.ReadFile(learnPath)
	if e = atomicWrite(filepath.Join(backup, "SKILL.md"), oldSkill, 0600); e != nil {
		return 0, e
	}
	if len(oldLearn) > 0 {
		if e = atomicWrite(filepath.Join(backup, "follow-up-learnings.md"), oldLearn, 0600); e != nil {
			return 0, e
		}
	}
	if e = atomicWrite(learnPath, learning, 0644); e != nil {
		return 0, e
	}
	if e = atomicWrite(skillPath, skill, 0644); e != nil {
		if len(oldLearn) > 0 {
			_ = atomicWrite(learnPath, oldLearn, 0644)
		} else {
			_ = os.Remove(learnPath)
		}
		return 0, e
	}
	for _, v := range chosen {
		s.job(v.JobID).AppliedRule = v.Rule
	}
	if e = s.saveLocked(false); e != nil {
		return 0, e
	}
	return len(chosen), nil
}

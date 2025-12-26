import { describe, expect, it } from 'vitest';

import { skillsCatalog } from '@/lib/skillsCatalog';

describe('skillsCatalog', () => {
  it('loads built-in skills with parsed metadata', () => {
    const names = skillsCatalog.skills.map((skill) => skill.name);

    expect(names).toContain('ppt-deck');
    expect(names).toContain('video-production');
    expect(names).toContain('email-drafting');
    expect(names).toContain('meeting-notes');
    expect(names).toContain('research-briefing');

    skillsCatalog.skills.forEach((skill) => {
      expect(skill.name.trim().length).toBeGreaterThan(0);
      expect(skill.title.trim().length).toBeGreaterThan(0);
      expect(skill.markdown.trim().length).toBeGreaterThan(0);
    });
  });
});

import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import { SkillsPanel } from '../SkillsPanel';

describe('SkillsPanel', () => {
  it('renders skills and expands a playbook', async () => {
    const user = userEvent.setup();
    render(<SkillsPanel />);

    expect(screen.getByText('Skills')).toBeInTheDocument();
    expect(screen.getAllByText('ppt-deck').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('video-production').length).toBeGreaterThanOrEqual(1);

    // Click the ppt-deck skill description to expand its markdown body
    const descEl = screen.getByText(/PPT 产出/);
    await user.click(descEl);
    // Expanded content should show markdown rendered from the skill catalog
    expect(screen.getByText(/presentation decks/i)).toBeInTheDocument();
  });
});

import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import { SkillsPanel } from '../SkillsPanel';

describe('SkillsPanel', () => {
  it('renders skills and expands a playbook', async () => {
    const user = userEvent.setup();
    render(<SkillsPanel />);

    expect(screen.getByText('Skills')).toBeInTheDocument();
    expect(screen.getByText('ppt_deck')).toBeInTheDocument();
    expect(screen.getByText('video_production')).toBeInTheDocument();

    await user.click(screen.getByText('PPT 产出（从目标到可交付 Deck）'));
    expect(screen.getByText(/何时使用/)).toBeInTheDocument();
  });
});

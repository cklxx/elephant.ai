import { act, render, screen } from '@testing-library/react';
import { EnvironmentStrip } from '../EnvironmentStrip';
import { handleEnvironmentSnapshot, resetDiagnostics } from '@/hooks/useDiagnostics';

describe('EnvironmentStrip', () => {
  beforeEach(() => {
    resetDiagnostics();
  });

  it('renders formatted environment metadata', () => {
    act(() => {
      handleEnvironmentSnapshot({
        event_type: 'workflow.diagnostic.environment_snapshot',
        timestamp: new Date().toISOString(),
        agent_level: 'core',
        captured: new Date().toISOString(),
        host: { HOSTNAME: 'host.local', USER: 'operator' },
      });
    });

    render(<EnvironmentStrip />);

    const strip = screen.getByTestId('environment-strip');
    expect(strip).toHaveTextContent('Host: HOSTNAME=host.local · USER=operator');
  });

  it('returns null when no metadata available', () => {
    const { container } = render(<EnvironmentStrip />);
    expect(container.firstChild).toBeNull();
  });
});

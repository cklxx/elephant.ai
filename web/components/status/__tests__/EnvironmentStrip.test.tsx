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
        event_type: 'environment_snapshot',
        timestamp: new Date().toISOString(),
        agent_level: 'core',
        captured: new Date().toISOString(),
        host: { HOSTNAME: 'host.local', USER: 'operator' },
        sandbox: { HOSTNAME: 'sandbox.local', SANDBOX_BASE_URL: 'http://sandbox' },
      });
    });

    render(<EnvironmentStrip />);

    const strip = screen.getByTestId('environment-strip');
    expect(strip).toHaveTextContent('Host: HOSTNAME=host.local · USER=operator');
    expect(strip).toHaveTextContent('Sandbox: HOSTNAME=sandbox.local · SANDBOX_BASE_URL=http://sandbox');
  });

  it('returns null when no metadata available', () => {
    const { container } = render(<EnvironmentStrip />);
    expect(container.firstChild).toBeNull();
  });
});

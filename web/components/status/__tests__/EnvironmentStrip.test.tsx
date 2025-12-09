import { act, render, screen } from '@testing-library/react';
import { EnvironmentStrip } from '../EnvironmentStrip';
import { handleEnvironmentSnapshot, resetDiagnostics } from '@/hooks/useDiagnostics';
import { handleSandboxProgress, resetSandboxProgress } from '@/hooks/useSandboxProgress';

describe('EnvironmentStrip', () => {
  beforeEach(() => {
    resetDiagnostics();
    resetSandboxProgress();
  });

  it('renders formatted environment metadata', () => {
    act(() => {
      handleEnvironmentSnapshot({
        event_type: 'workflow.diagnostic.environment_snapshot',
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

  it('shows progress message when sandbox is initializing', () => {
    act(() => {
      handleSandboxProgress({
        event_type: 'workflow.diagnostic.sandbox_progress',
        timestamp: new Date().toISOString(),
        agent_level: 'core',
        status: 'running',
        stage: 'health_check',
        message: 'Verifying sandbox connectivity',
        step: 2,
        total_steps: 3,
        updated: new Date().toISOString(),
      });
    });

    render(<EnvironmentStrip />);

    const strip = screen.getByTestId('environment-strip');
    expect(strip).toHaveTextContent('Sandbox initializing (2/3): Verifying sandbox connectivity');
  });

  it('prefers progress message when sandbox fails', () => {
    act(() => {
      handleSandboxProgress({
        event_type: 'workflow.diagnostic.sandbox_progress',
        timestamp: new Date().toISOString(),
        agent_level: 'core',
        status: 'error',
        stage: 'health_check',
        message: 'Sandbox unreachable - check SANDBOX_BASE_URL',
        step: 2,
        total_steps: 2,
        error: 'Sandbox unreachable - check SANDBOX_BASE_URL',
        updated: new Date().toISOString(),
      });
    });

    render(<EnvironmentStrip />);

    const strip = screen.getByTestId('environment-strip');
    expect(strip).toHaveTextContent('Sandbox error: Sandbox unreachable - check SANDBOX_BASE_URL');
  });
});

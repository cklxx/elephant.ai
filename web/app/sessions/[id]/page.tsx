import { SessionDetailsClient } from './SessionDetailsClient';

type SessionPageProps = {
  params: {
    id: string;
  };
};

export default function SessionDetailsPage({ params }: SessionPageProps) {
  return <SessionDetailsClient sessionId={params.id} />;
}

export async function generateStaticParams(): Promise<{ id: string }[]> {
  return [{ id: 'demo-session' }];
}

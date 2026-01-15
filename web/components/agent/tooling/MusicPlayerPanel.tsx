'use client';

import { useEffect, useMemo, useRef, useState } from 'react';
import Image from 'next/image';
import { SimplePanel, PanelHeader } from './ToolPanels';
import { cn } from '@/lib/utils';

type MusicTrack = {
  title?: string;
  artist?: string;
  album?: string;
  preview_url?: string;
  track_url?: string;
  artwork_url?: string;
};

type MusicPlayerPanelProps = {
  query: string;
  tracks: MusicTrack[];
};

export function MusicPlayerPanel({ query, tracks }: MusicPlayerPanelProps) {
  const audioRef = useRef<HTMLAudioElement | null>(null);
  const [activeIndex, setActiveIndex] = useState(0);
  const currentTrack = tracks[activeIndex];
  const heading = query ? `音乐播放：${query}` : '音乐播放';

  const playableTracks = useMemo(
    () => tracks.filter((track) => Boolean(track.preview_url)),
    [tracks],
  );

  useEffect(() => {
    if (!audioRef.current || !currentTrack?.preview_url) return;
    audioRef.current.play().catch(() => undefined);
  }, [currentTrack?.preview_url]);

  if (playableTracks.length === 0 || !currentTrack?.preview_url) {
    return null;
  }

  return (
    <SimplePanel>
      <PanelHeader title={heading} />
      <div className="space-y-3">
        <div className="flex flex-wrap items-center gap-3">
          {currentTrack.artwork_url ? (
            <Image
              src={currentTrack.artwork_url}
              alt={currentTrack.title || 'Album art'}
              width={64}
              height={64}
              className="h-16 w-16 rounded-lg border border-border object-cover"
            />
          ) : null}
          <div className="min-w-0">
            <div className="text-sm font-semibold text-foreground">
              {currentTrack.title || 'Unknown track'}
            </div>
            <div className="text-xs text-muted-foreground">
              {[currentTrack.artist, currentTrack.album].filter(Boolean).join(' · ')}
            </div>
          </div>
          {currentTrack.track_url ? (
            <a
              className="text-xs text-primary underline-offset-4 hover:underline"
              href={currentTrack.track_url}
              target="_blank"
              rel="noreferrer"
            >
              详情
            </a>
          ) : null}
        </div>
        <audio
          ref={audioRef}
          src={currentTrack.preview_url}
          controls
          autoPlay
          className="w-full"
        />
        <div className="space-y-1">
          {playableTracks.map((track, index) => (
            <button
              key={`${track.title ?? 'track'}-${index}`}
              type="button"
              onClick={() => setActiveIndex(index)}
              className={cn(
                'flex w-full items-center justify-between gap-3 rounded-md border px-3 py-2 text-left text-xs transition',
                index === activeIndex
                  ? 'border-primary/40 bg-primary/10 text-primary'
                  : 'border-border/60 bg-background hover:bg-muted',
              )}
            >
              <span className="truncate">
                {track.title || 'Unknown track'}
                {track.artist ? ` · ${track.artist}` : ''}
              </span>
              <span className="shrink-0 text-[10px] uppercase tracking-wide opacity-60">
                {index === activeIndex ? 'Playing' : 'Play'}
              </span>
            </button>
          ))}
        </div>
      </div>
    </SimplePanel>
  );
}

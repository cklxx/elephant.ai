"use client";

import { cn } from "@/lib/utils";

interface MagicBlackHoleProps {
    className?: string;
    size?: "sm" | "md" | "lg";
}

export function MagicBlackHole({ className, size = "md" }: MagicBlackHoleProps) {
    return (
        <div className={cn("relative flex items-center justify-center", className)}>
            {/* Core Black Hole */}
            <div className="relative z-10">
                <div
                    className={cn(
                        "rounded-full bg-black shadow-[0_0_40px_-5px_#7c3aed]",
                        size === "sm" && "w-8 h-8",
                        size === "md" && "w-16 h-16",
                        size === "lg" && "w-32 h-32"
                    )}
                />
                {/* Accretion Disk (Spinning) */}
                <div
                    className={cn(
                        "absolute inset-[-50%] animate-[spin_3s_linear_infinite] rounded-full border-t-2 border-r-2 border-transparent border-t-purple-500 border-r-fuchsia-500 opacity-80 blur-[2px]",
                        size === "sm" && "border-2",
                        size === "md" && "border-4",
                        size === "lg" && "border-8"
                    )}
                />
                {/* Particle Horizon (Counter-Spinning) */}
                <div
                    className={cn(
                        "absolute inset-[-20%] animate-[spin_4s_linear_infinite_reverse] rounded-full border-b-2 border-l-2 border-transparent border-b-indigo-500 border-l-violet-500 opacity-60 blur-[1px]",
                        size === "sm" && "border-2",
                        size === "md" && "border-4",
                        size === "lg" && "border-8"
                    )}
                />
            </div>

            {/* Ambient Glow */}
            <div className="absolute inset-0 bg-purple-900/20 blur-3xl rounded-full transform scale-150 animate-pulse" />
        </div>
    );
}

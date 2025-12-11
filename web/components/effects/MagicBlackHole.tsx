"use client";

import { cn } from "@/lib/utils";

interface MagicBlackHoleProps {
    className?: string;
    size?: "sm" | "md" | "lg";
}

export function MagicBlackHole({ className, size = "md" }: MagicBlackHoleProps) {
    return (
        <div className={cn("relative flex items-center justify-center", className)}>
            <style jsx>{`
                @keyframes implode {
                    0% { transform: scale(3); opacity: 0; }
                    50% { opacity: 0.5; }
                    100% { transform: scale(0); opacity: 0; }
                }
            `}</style>

            {/* Core Black Hole - Reduced Sizes */}
            <div className="relative z-10 flex items-center justify-center">
                <div
                    className={cn(
                        "rounded-full bg-black shadow-[inset_0_0_20px_0px_#4c1d95]",
                        size === "sm" && "w-6 h-6",
                        size === "md" && "w-10 h-10", // Reduced from w-16 h-16
                        size === "lg" && "w-24 h-24"
                    )}
                />

                {/* Imploding Particles/Rings (The Sucking Effect) */}
                {[...Array(3)].map((_, i) => (
                    <div
                        key={i}
                        className={cn(
                            "absolute rounded-full border border-purple-500/30",
                            size === "sm" && "w-4 h-4",
                            size === "md" && "w-8 h-8",
                            size === "lg" && "w-20 h-20"
                        )}
                        style={{
                            animation: `implode 2s linear infinite`,
                            animationDelay: `${i * 0.6}s`
                        }}
                    />
                ))}

                {/* Accretion Disk (Spinning Tighter) */}
                <div
                    className={cn(
                        "absolute -inset-1 animate-[spin_3s_linear_infinite] rounded-full border-t border-purple-400/50 opacity-80 blur-[1px]",
                        size === "sm" && "border-t-[1px]",
                        size === "md" && "border-t-[2px]"
                    )}
                />
            </div>

            {/* Ambient Glow (Reduced) */}
            <div className="absolute inset-0 bg-purple-900/40 blur-xl rounded-full transform scale-125 animate-pulse" />
        </div>
    );
}

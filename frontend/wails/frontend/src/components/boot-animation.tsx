// Kron GUI — boot animation.
// Plays a short fade-in brand + progress bar sequence,
// then fires `onDone` so the main shell can mount.

import {useEffect, useState} from 'react';
import {Sparkles} from 'lucide-react';
import {Progress} from '@/components/ui/progress';

interface BootAnimationProps {
    onDone: () => void;
    durationMs?: number;
}

export function BootAnimation(props: BootAnimationProps): JSX.Element {
    const {onDone, durationMs = 1600} = props;
    const [progress, setProgress] = useState<number>(0);
    const [phase, setPhase] = useState<'in' | 'hold' | 'out'>('in');

    useEffect(() => {
        const tickMs = 30;
        const steps = durationMs / tickMs;
        const inc = 100 / steps;
        const id = setInterval(() => {
            setProgress((p) => {
                const next = p + inc;
                if (next >= 100) {
                    clearInterval(id);
                    setPhase('hold');
                    return 100;
                }
                return next;
            });
        }, tickMs);
        return (): void => clearInterval(id);
    }, [durationMs]);

    // After we hit 100, hold briefly, then fade out and notify.
    useEffect(() => {
        if (phase !== 'hold') return;
        const id = setTimeout(() => {
            setPhase('out');
            // Wait the CSS fade-out (450ms) before unmounting.
            setTimeout(onDone, 450);
        }, 220);
        return (): void => clearTimeout(id);
    }, [phase, onDone]);

    return (
        <div
            aria-hidden={phase === 'out'}
            className={[
                'fixed inset-0 z-50',
                'flex flex-col items-center justify-center',
                'bg-background',
                'transition-opacity duration-500',
                phase === 'out'
                    ? 'opacity-0 pointer-events-none'
                    : 'opacity-100',
            ].join(' ')}
        >
            <div className="flex flex-col items-center gap-6 w-72">
                <div
                    className={[
                        'w-16 h-16 rounded-2xl',
                        'bg-primary text-primary-foreground',
                        'flex items-center justify-center',
                        'transition-transform duration-700',
                        phase === 'in' ? 'scale-90' : 'scale-100',
                    ].join(' ')}
                >
                    <Sparkles className="h-8 w-8"/>
                </div>

                <div className="text-center space-y-1">
                    <div className="text-lg font-semibold tracking-tight">
                        Kron
                    </div>
                    <div className="text-xs text-muted-foreground">
                        Loading intent ledger…
                    </div>
                </div>

                <Progress value={progress} className="h-1"/>
            </div>
        </div>
    );
}

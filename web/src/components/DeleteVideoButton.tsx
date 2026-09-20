import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { api } from '../api';

type Props = {
  videoId: string;
  /** "card" sits absolutely on the thumbnail. "inline" is a normal pill button. */
  variant?: 'card' | 'inline';
  redirectHome?: boolean;
};

export function DeleteVideoButton({ videoId, variant = 'inline', redirectHome = false }: Props) {
  const navigate = useNavigate();
  const [pending, setPending] = useState(false);

  const onClick = async (e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    if (pending) return;
    if (!confirm('Delete this karaoke?')) return;
    setPending(true);
    try {
      await api.deleteVideo(videoId);
      if (redirectHome) navigate('/');
    } catch (err) {
      alert(err instanceof Error ? err.message : String(err));
    } finally {
      setPending(false);
    }
  };

  const cls =
    variant === 'card'
      ? 'absolute left-2 top-2 z-10 bg-bg/90 px-2 py-1 text-[10px] uppercase tracking-[0.18em] text-ink opacity-0 transition-opacity hover:text-accent group-hover:opacity-100'
      : 'border border-line px-3 py-1 text-[10px] uppercase tracking-[0.18em] text-mute transition-colors hover:border-accent hover:text-accent';

  return (
    <button type="button" onClick={onClick} disabled={pending} className={cls}>
      {pending ? 'deleting…' : 'delete'}
    </button>
  );
}

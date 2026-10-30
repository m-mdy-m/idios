import { useState } from 'react';
import { Eye, EyeOff } from 'lucide-react';

type PrivatePathProps = {
  value?: string | null;
  className?: string;
  hiddenLabel?: string;
};

/** Filesystem locations are hidden by default to avoid leaking personal paths in screenshots. */
export function PrivatePath({ value, className = '', hiddenLabel = 'Hidden for privacy' }: PrivatePathProps) {
  const [revealed, setRevealed] = useState(false);
  const path = value?.trim() ?? '';

  return <span className={`private-path ${className}`.trim()}>
    <code>{revealed && path ? path : path ? hiddenLabel : 'Not configured'}</code>
    {path && <button
      type="button"
      className="private-path__toggle"
      onClick={() => setRevealed((current) => !current)}
      aria-label={revealed ? 'Hide filesystem path' : 'Reveal filesystem path'}
      title={revealed ? 'Hide path' : 'Reveal path'}
    >{revealed ? <EyeOff size={13} /> : <Eye size={13} />}</button>}
  </span>;
}

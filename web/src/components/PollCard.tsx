import { useVotePoll } from '@/hooks/usePost';
import type { NewsLink, Poll } from '@/types/api';
import { cn } from '@/lib/utils';
import { toast } from 'sonner';

export default function PollCard({ poll, postId }: { poll: Poll; postId: number }) {
  const vote = useVotePoll();
  const voted = poll.selected_option_id != null;
  return (
    <div className="mt-3 space-y-2 rounded-xl border border-border p-3" onClick={(event) => event.stopPropagation()}>
      {poll.options.map((option) => {
        const percentage = poll.total_votes ? Math.round((option.vote_count / poll.total_votes) * 100) : 0;
        return (
          <button
            key={option.id}
            disabled={voted || poll.closed || vote.isPending}
            onClick={() => vote.mutate({ postId, optionId: option.id }, { onError: () => toast.error('Could not record your vote') })}
            className={cn('relative w-full overflow-hidden rounded-lg border border-border p-3 text-left text-sm', !voted && !poll.closed && 'hover:border-primary', poll.selected_option_id === option.id && 'border-primary')}
          >
            {(voted || poll.closed) && <span className="absolute inset-y-0 left-0 bg-primary/10" style={{ width: `${percentage}%` }} />}
            <span className="relative flex justify-between gap-2"><span>{option.label}</span>{(voted || poll.closed) && <span>{percentage}%</span>}</span>
          </button>
        );
      })}
      {poll.closed && <p className="text-xs text-muted-foreground">Poll closed</p>}
      <span className="block text-right text-xs text-muted-foreground">{poll.total_votes} votes</span>
    </div>
  );
}

export function NewsCard({ news }: { news?: NewsLink }) {
  if (!news) return null;
  return (
    <a
      href={news.url}
      target="_blank"
      rel="noopener noreferrer"
      onClick={(event) => event.stopPropagation()}
      className="mt-3 block overflow-hidden rounded-xl border border-border"
    >
      {news.image_url && <img src={news.image_url} alt="" className="aspect-video w-full object-cover" onError={(event) => (event.currentTarget.style.display = 'none')} />}
      <div className="p-3">
        {news.site_name && <p className="text-xs font-medium text-muted-foreground">{news.site_name}</p>}
        <p className="mt-1 text-sm font-semibold leading-snug">{news.title || news.url}</p>
      </div>
    </a>
  );
}

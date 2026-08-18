import { useState } from 'react';
import { Link } from 'react-router-dom';
import { useMyLists, useCreateList, useDeleteList } from '@/hooks/useLists';
import { Button } from '@/components/ui/button';
import { Dialog, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog';
import { CustomDialogContent } from '@/components/ui/custom-dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { List as ListIcon, Loader2, Plus, Trash2, Users } from 'lucide-react';
import { toast } from 'sonner';
import ConfirmDialog from '@/components/ConfirmDialog';

const ListsPage: React.FC = () => {
  const { data, isLoading } = useMyLists();
  const createList = useCreateList();
  const deleteList = useDeleteList();
  const lists = data?.data ?? [];

  const [open, setOpen] = useState(false);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [deleteTarget, setDeleteTarget] = useState<number | null>(null);

  const submit = () => {
    if (!name.trim()) return;
    createList.mutate({ name: name.trim(), description: description.trim() }, {
      onSuccess: () => { setOpen(false); setName(''); setDescription(''); toast.success('List created'); },
      onError: () => toast.error('Failed to create list'),
    });
  };

  const remove = (id: number) => {
    deleteList.mutate(id, {
      onSuccess: () => toast.success('List deleted'),
      onError: () => toast.error('Failed to delete list'),
    });
  };

  return (
    <div className="mx-auto w-full max-w-xl pt-6">
      <header className="px-4 pb-4 flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-primary">Lists</h1>
          <p className="text-sm text-muted-foreground">Collections of accounts, curated by you.</p>
        </div>
        <Button onClick={() => setOpen(true)}><Plus className="h-4 w-4 mr-2" />New list</Button>
      </header>

      {isLoading ? (
        <div className="flex justify-center py-12"><Loader2 className="h-6 w-6 animate-spin" /></div>
      ) : lists.length === 0 ? (
        <div className="flex flex-col items-center gap-3 p-12 text-center text-muted-foreground">
          <ListIcon className="h-10 w-10" />
          <p>You don't have any lists yet.</p>
        </div>
      ) : (
        <div className="space-y-2 px-4 pb-8">
          {lists.map((list) => (
            <Link
              key={list.id}
              to={`/lists/${list.id}`}
              className="flex items-center justify-between rounded-xl border border-border p-4 hover:bg-muted"
            >
              <div className="min-w-0">
                <p className="font-semibold text-primary">{list.name}</p>
                <p className="text-sm text-muted-foreground truncate">{list.description || 'No description'}</p>
                <p className="text-xs text-muted-foreground mt-1 flex items-center gap-1">
                  <Users className="h-3 w-3" /> {list.member_count} members
                </p>
              </div>
              <Button variant="ghost" size="icon" className="text-destructive" onClick={(e) => { e.preventDefault(); setDeleteTarget(list.id); }}>
                <Trash2 className="h-4 w-4" />
              </Button>
            </Link>
          ))}
        </div>
      )}

      <Dialog open={open} onOpenChange={setOpen}>
        <CustomDialogContent className="sm:max-w-md bg-card">
          <DialogHeader><DialogTitle className="text-primary">New list</DialogTitle></DialogHeader>
          <div className="space-y-3">
            <div className="space-y-2">
              <Label htmlFor="list-name">Name</Label>
              <Input id="list-name" value={name} onChange={(e) => setName(e.target.value)} placeholder="Go developers" maxLength={100} />
            </div>
            <div className="space-y-2">
              <Label htmlFor="list-desc">Description</Label>
              <Textarea id="list-desc" value={description} onChange={(e) => setDescription(e.target.value)} placeholder="What's this list about?" maxLength={300} className="min-h-20" />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setOpen(false)}>Cancel</Button>
            <Button disabled={!name.trim() || createList.isPending} onClick={submit}>
              {createList.isPending && <Loader2 className="h-4 w-4 animate-spin mr-2" />}
              Create
            </Button>
          </DialogFooter>
        </CustomDialogContent>
      </Dialog>

      <ConfirmDialog
        open={deleteTarget !== null}
        title="Delete this list?"
        description="This will permanently remove the list and its members."
        confirmLabel="Delete"
        onConfirm={() => deleteTarget !== null && remove(deleteTarget)}
        onOpenChange={(o) => !o && setDeleteTarget(null)}
      />
    </div>
  );
};

export default ListsPage;
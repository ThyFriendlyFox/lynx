# Use cases

Each case has the same shape. **Share** is what the first person does.
**Join** is what the second person does. **Result** is what both people see
afterwards.

## Send a folder to a friend

**Share.** The person right clicks the folder and chooses "Share with Lynx".
Lynx copies a code to the clipboard.
**Join.** The friend runs `lynx join <code>` in an empty folder.
**Result.** Both folders hold the same files. A change on one side reaches the
other side in about 1 second.

## Work on a document together

**Share.** Two people already share a folder.
**Join.** Both people edit files in it during the same hour.
**Result.** Each save reaches the other computer. When both people change the
same file at the same time, Lynx keeps both copies: the file, and a second
file named `<name> (conflict from <peer> <date>)`.

## Contribute without git

**Share.** A maintainer shares the project folder with Lynx.
**Join.** A writer with no git account joins the folder and edits the text.
**Result.** The maintainer sees the edit at once and commits it with the tool
the maintainer already uses. The writer never learns git.

## Keep a folder on two of your own computers

**Share.** The person shares a folder on the desktop computer.
**Join.** The person joins the same code on the laptop.
**Result.** Both computers hold the folder. The laptop catches up when it
comes back online.

## Move a large file to another person

**Share.** The person drops a 2 GB video into a shared folder.
**Join.** The friend already holds the folder.
**Result.** The bytes travel directly between the 2 computers. No upload to a
service and no size limit under 2 GB.

## Recover a file that a peer replaced

**Share.** Two people share a folder.
**Join.** One person overwrites a file that the other person wrote.
**Result.** The old bytes are in `.lynx/trash/`. The person copies the file
back into the folder.

## Keep private files out of the folder

**Share.** The person shares a project folder that holds a `secret/` folder.
**Join.** The person writes `secret/` into `.lynxignore` before the friend
joins.
**Result.** The friend never receives the private files. The rest of the
folder syncs.

## Ask a question about the folder

**Share.** A person shares a folder that holds many documents.
**Join.** The person runs `lynx chat` in the folder.
**Result.** The person asks what is in the folder and gets an answer about
these files. This case is on the roadmap and does not work yet.

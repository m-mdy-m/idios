# Shelf: books, PDFs, notes, and highlights

Shelf is your local reading library. It connects folders that already exist on your computer; it does not need to copy your books into the IDIOS database.

## 1. Prepare a book folder

Create a parent library folder and put one folder inside it for each book. Keep the original PDF in the book folder. You may add a `README.md` file with a short description or reading context.

Example:

```text
Books/
  A Theory of Justice/
    book.pdf
    README.md
  Clean Code/
    clean-code.pdf
```

## 2. Connect the library

1. Open **Shelf**.
2. Choose the action to connect or add a library folder.
3. Select the parent folder that contains your book folders.
4. Let Shelf scan the available folders and files.
5. Open a book to review its metadata and reading progress.

The browser asks for access through the local IDIOS file system integration. Keep the library in a location you trust.

## 3. Find a book

Use the search field for a title or author. Use the status filter to show books that are To Read, Reading, or Finished. Open advanced filters for topic, file type, and sort order. Active filters appear as removable chips. Switch between grid and list view when it helps.

## 4. Read a PDF

Open a book and choose **Read PDF**. The Reader provides page navigation, zoom, text selection, in-document search, and reading progress. The PDF remains in its original folder.

When text is selected, save it as a highlight or attach a note. Saved annotations are stored by IDIOS and reappear when you open the same book again. A note can appear as an underline with its note text available from the annotation.

## 5. Use notes and questions

Book notes are separate from PDF annotations:

- A **book note** is a longer note attached to the book.
- A **PDF annotation** belongs to a selected passage and page.
- A **question** records something you want to investigate or discuss.
- A **quote** is an excerpt you want to preserve as a reusable item.

Use the book tabs to move between Overview, Notes, Highlights, Questions, and Tags. Link a companion IDIOS Repository when you want to turn reading into practical work.

## 6. Use AI while reading

The in-reader Tutor and discussion tools can use the current book context, selected passage, saved annotations, and selected reading skills. AI suggestions are not saved automatically unless you choose a save action and IDIOS confirms that it succeeded.

For model setup and privacy behavior, read the [AI guide](ai.md).

## Troubleshooting

- **Book is not shown:** confirm that the selected library folder contains book subfolders and supported PDF/Markdown files.
- **PDF is blank:** update to the current IDIOS release; PDF.js worker assets are bundled with the Web build.
- **Highlight does not appear after reopening:** confirm that IDIOS reported the annotation as saved and that you reopened the same book record.
- **Reader cannot find the source file:** keep the original folder in place and reconnect the library if the path changed.

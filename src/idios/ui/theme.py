"""Small reusable dark UI toolkit for optional IDIOS desktop windows.

The core application remains terminal-first and dependency-free.  These helpers
standardize spacing, colors, cards, scrolling, and buttons for any future Tk UI.
"""
from __future__ import annotations

from collections.abc import Callable
try:
    import tkinter as tk
except ImportError:  # Tk can be omitted from minimal Python/Linux installations.
    tk = None  # type: ignore[assignment]

COLORS: dict[str, str] = {
    "background": "#0B1020",
    "surface": "#11192B",
    "card": "#141E32",
    "card_hover": "#1B2942",
    "item": "#10182A",
    "border": "#27344D",
    "border_subtle": "#202B41",
    "text": "#F2F5FC",
    "muted": "#9AA8C0",
    "faint": "#71809B",
    "accent": "#9485FF",
    "accent_hover": "#A79BFF",
    "accent_soft": "#28264F",
    "teal": "#55D6C2",
    "teal_soft": "#153B40",
    "green": "#5ED9A1",
    "green_soft": "#163D36",
    "amber": "#FFBF69",
    "amber_soft": "#49351F",
    "red": "#FF7A91",
    "red_soft": "#482633",
    "white": "#FFFFFF",
}

FONTS: dict[str, tuple[str, int, str]] = {
    "brand": ("Segoe UI", 17, "bold"),
    "eyebrow": ("Segoe UI", 8, "bold"),
    "title": ("Segoe UI", 24, "bold"),
    "heading": ("Segoe UI", 12, "bold"),
    "body": ("Segoe UI", 10, "normal"),
    "body_bold": ("Segoe UI", 10, "bold"),
    "small": ("Segoe UI", 9, "normal"),
    "tiny": ("Segoe UI", 8, "normal"),
    "metric": ("Segoe UI", 25, "bold"),
}


def configure_window(window: tk.Tk | tk.Toplevel, title: str) -> None:
    """Apply the shared window baseline and best-effort native dark title bar."""
    window.title(title)
    window.configure(background=COLORS["background"])
    window.option_add("*Font", FONTS["body"])
    _enable_dark_titlebar(window)


def _enable_dark_titlebar(window: tk.Misc) -> None:
    """Enable the Windows 10/11 dark title bar where DWM supports it."""
    try:
        import ctypes
        import sys

        if sys.platform != "win32":
            return
        window.update_idletasks()
        hwnd = ctypes.windll.user32.GetParent(window.winfo_id()) or window.winfo_id()
        value = ctypes.c_int(1)
        # 20 is DWMWA_USE_IMMERSIVE_DARK_MODE on recent Windows 10 builds;
        # 19 is supported by some earlier builds.
        for attribute in (20, 19):
            result = ctypes.windll.dwmapi.DwmSetWindowAttribute(
                hwnd, attribute, ctypes.byref(value), ctypes.sizeof(value)
            )
            if result == 0:
                break
    except (AttributeError, OSError, tk.TclError):
        # The title bar is an OS surface. A failure here must never block the UI.
        return


def make_card(parent: tk.Misc, *, padding: int = 16, background: str | None = None) -> tuple[tk.Frame, tk.Frame]:
    """Create a bordered card and its padded content frame."""
    card = tk.Frame(
        parent,
        background=background or COLORS["card"],
        highlightbackground=COLORS["border"],
        highlightcolor=COLORS["border"],
        highlightthickness=1,
        bd=0,
    )
    content = tk.Frame(card, background=background or COLORS["card"], bd=0)
    content.pack(fill="both", expand=True, padx=padding, pady=padding)
    return card, content


def make_label(
    parent: tk.Misc,
    text: str,
    *,
    font: tuple[str, int, str] | None = None,
    color: str | None = None,
    background: str | None = None,
    **kwargs: object,
) -> tk.Label:
    """Create a label using the shared palette and typography."""
    return tk.Label(
        parent,
        text=text,
        font=font or FONTS["body"],
        fg=color or COLORS["text"],
        bg=background or parent.cget("bg"),
        bd=0,
        highlightthickness=0,
        **kwargs,
    )


def make_button(
    parent: tk.Misc,
    text: str,
    command: Callable[[], object],
    *,
    primary: bool = False,
) -> tk.Button:
    """Create a flat, keyboard-friendly button with a subtle hover state."""
    normal = COLORS["accent"] if primary else COLORS["card"]
    hovered = COLORS["accent_hover"] if primary else COLORS["card_hover"]
    foreground = COLORS["background"] if primary else COLORS["text"]
    button = tk.Button(
        parent,
        text=text,
        command=command,
        font=FONTS["body_bold"],
        fg=foreground,
        bg=normal,
        activeforeground=foreground,
        activebackground=hovered,
        relief="flat",
        bd=0,
        padx=18,
        pady=10,
        cursor="hand2",
        takefocus=True,
        highlightthickness=1,
        highlightbackground=normal,
        highlightcolor=COLORS["accent_hover"],
    )
    button.bind("<Enter>", lambda _event: button.configure(bg=hovered))
    button.bind("<Leave>", lambda _event: button.configure(bg=normal))
    return button


def make_pill(parent: tk.Misc, text: str, *, foreground: str, background: str) -> tk.Label:
    """Render a compact status/count chip."""
    return tk.Label(
        parent,
        text=text,
        font=FONTS["eyebrow"],
        fg=foreground,
        bg=background,
        bd=0,
        padx=9,
        pady=5,
    )


def make_brand_mark(parent: tk.Misc, size: int = 42) -> tk.Canvas:
    """Draw the IDIOS book/page mark without requiring image assets."""
    canvas = tk.Canvas(
        parent,
        width=size,
        height=size,
        bg=COLORS["accent_soft"],
        highlightthickness=0,
        bd=0,
    )
    scale = size / 42
    stroke = max(1, round(1.8 * scale))
    canvas.create_rectangle(
        10 * scale, 10 * scale, 32 * scale, 32 * scale,
        outline=COLORS["accent"], width=stroke,
    )
    canvas.create_line(
        21 * scale, 11 * scale, 21 * scale, 31 * scale,
        fill=COLORS["accent"], width=stroke,
    )
    for y in (16, 21, 26):
        canvas.create_line(
            14 * scale, y * scale, 18 * scale, y * scale,
            fill=COLORS["white"], width=stroke, capstyle="round",
        )
        canvas.create_line(
            24 * scale, y * scale, 28 * scale, y * scale,
            fill=COLORS["white"], width=stroke, capstyle="round",
        )
    return canvas


class ScrollableFrame:
    """A dark, resize-aware scrolling container reusable across IDIOS windows."""

    def __init__(self, parent: tk.Misc, *, background: str | None = None) -> None:
        self.background = background or COLORS["background"]
        self.frame = tk.Frame(parent, background=self.background, bd=0)
        self.canvas = tk.Canvas(
            self.frame,
            background=self.background,
            highlightthickness=0,
            bd=0,
        )
        self.scrollbar = tk.Scrollbar(
            self.frame,
            orient="vertical",
            command=self.canvas.yview,
            bg=COLORS["surface"],
            activebackground=COLORS["accent"],
            troughcolor=self.background,
            bd=0,
            relief="flat",
            width=9,
            highlightthickness=0,
        )
        self.canvas.configure(yscrollcommand=self.scrollbar.set)
        self.scrollbar.pack(side="right", fill="y", padx=(8, 0))
        self.canvas.pack(side="left", fill="both", expand=True)
        self.content = tk.Frame(self.canvas, background=self.background, bd=0)
        self._window = self.canvas.create_window((0, 0), window=self.content, anchor="nw")
        self.content.bind("<Configure>", self._sync_scroll_region)
        self.canvas.bind("<Configure>", self._sync_content_width)

    def bind_mousewheel_to_children(self) -> None:
        """Bind scrolling to this view's widgets without leaking global bindings."""
        def bind_tree(widget: tk.Misc) -> None:
            widget.bind("<MouseWheel>", self._on_mousewheel, add="+")
            widget.bind("<Button-4>", self._on_linux_wheel_up, add="+")
            widget.bind("<Button-5>", self._on_linux_wheel_down, add="+")
            for child in widget.winfo_children():
                bind_tree(child)

        bind_tree(self.canvas)
        bind_tree(self.content)

    def _sync_scroll_region(self, _event: tk.Event[tk.Misc]) -> None:
        self.canvas.configure(scrollregion=self.canvas.bbox("all"))

    def _sync_content_width(self, event: tk.Event[tk.Misc]) -> None:
        self.canvas.itemconfigure(self._window, width=event.width)

    def _on_mousewheel(self, event: tk.Event[tk.Misc]) -> str:
        delta = getattr(event, "delta", 0)
        if delta:
            # Windows uses multiples of 120; macOS often sends +/-1.
            units = -1 if delta > 0 else 1
            if abs(delta) >= 120:
                units = int(-delta / 120)
            self.canvas.yview_scroll(units, "units")
        return "break"

    def _on_linux_wheel_up(self, _event: tk.Event[tk.Misc]) -> str:
        self.canvas.yview_scroll(-1, "units")
        return "break"

    def _on_linux_wheel_down(self, _event: tk.Event[tk.Misc]) -> str:
        self.canvas.yview_scroll(1, "units")
        return "break"

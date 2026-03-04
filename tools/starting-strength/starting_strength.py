#!/usr/bin/env python3
"""
Starting Strength Workout Tracker - Terminal UI
Navigate with arrow keys, add reps with +/-, save automatically
"""

import curses
import json
import os
from datetime import datetime

EXERCISES = [
    {"name": "Pull-Ups", "target": 50, "id": "pullups"},
    {"name": "DB Overhead Press", "target": 50, "id": "ohp"},
    {"name": "DB Bent-Over Row", "target": 50, "id": "rows"},
    {"name": "DB Floor Press", "target": 50, "id": "floorpress"},
    {"name": "DB Romanian Deadlift", "target": 50, "id": "rdl"},
    {"name": "Goblet Squats", "target": 50, "id": "squats"},
    {"name": "Ab Roller", "target": 50, "id": "abroller"},
]

DATA_FILE = os.path.expanduser("~/.starting_strength.json")


def load_data():
    today = datetime.now().strftime("%Y-%m-%d")
    try:
        with open(DATA_FILE, 'r') as f:
            all_data = json.load(f)
            return all_data.get(today, {ex["id"]: 0 for ex in EXERCISES})
    except (FileNotFoundError, json.JSONDecodeError):
        return {ex["id"]: 0 for ex in EXERCISES}


def save_data(workout_data):
    today = datetime.now().strftime("%Y-%m-%d")
    try:
        with open(DATA_FILE, 'r') as f:
            all_data = json.load(f)
    except (FileNotFoundError, json.JSONDecodeError):
        all_data = {}

    all_data[today] = workout_data

    with open(DATA_FILE, 'w') as f:
        json.dump(all_data, f, indent=2)


def make_tally_marks(count):
    groups = count // 5
    remainder = count % 5
    marks = "𝍸 " * groups + "|" * remainder
    return marks if marks else "—"


def draw_progress_bar(window, y, x, width, current, target):
    if target == 0:
        percentage = 0
    else:
        percentage = min(current / target, 1.0)

    filled = int(width * percentage)

    bar = "█" * filled + "░" * (width - filled)
    color = 3 if current >= target else 2  # Green if complete, yellow otherwise

    window.addstr(y, x, "[", curses.color_pair(1))
    window.addstr(y, x + 1, bar, curses.color_pair(color))
    window.addstr(y, x + width + 1, "]", curses.color_pair(1))

    pct_text = f" {int(percentage * 100)}%"
    window.addstr(y, x + width + 2, pct_text, curses.color_pair(color))


def main(stdscr):
    """Main TUI loop"""
    # Initialize colors
    curses.start_color()
    curses.init_pair(1, curses.COLOR_WHITE, curses.COLOR_BLACK)
    curses.init_pair(2, curses.COLOR_YELLOW, curses.COLOR_BLACK)
    curses.init_pair(3, curses.COLOR_GREEN, curses.COLOR_BLACK)
    curses.init_pair(4, curses.COLOR_RED, curses.COLOR_BLACK)
    curses.init_pair(5, curses.COLOR_CYAN, curses.COLOR_BLACK)

    curses.curs_set(0)

    workout_data = load_data()
    selected = 0

    while True:
        stdscr.clear()
        height, width = stdscr.getmaxyx()

        # Header
        title = "🏋️  STARTING STRENGTH"
        date_str = datetime.now().strftime("%A, %B %d, %Y")

        stdscr.addstr(0, (width - len(title)) // 2, title, curses.color_pair(4) | curses.A_BOLD)
        stdscr.addstr(1, (width - len(date_str)) // 2, date_str, curses.color_pair(1))
        stdscr.addstr(2, 0, "─" * width, curses.color_pair(1))

        # Exercises
        y_offset = 4
        for idx, exercise in enumerate(EXERCISES):
            exercise_id = exercise["id"]
            name = exercise["name"]
            target = exercise["target"]
            current = workout_data.get(exercise_id, 0)

            is_selected = (idx == selected)

            name_str = f"{name}:"
            count_str = f"{current}/{target}"

            if is_selected:
                stdscr.addstr(y_offset, 2, "►", curses.color_pair(4) | curses.A_BOLD)
                name_color = curses.color_pair(5) | curses.A_BOLD
            else:
                name_color = curses.color_pair(1)

            stdscr.addstr(y_offset, 4, name_str, name_color)
            stdscr.addstr(y_offset, 30, count_str, curses.color_pair(3) | curses.A_BOLD)

            bar_width = min(40, width - 45)
            draw_progress_bar(stdscr, y_offset + 1, 4, bar_width, current, target)

            tally = make_tally_marks(current)
            stdscr.addstr(y_offset + 2, 4, tally[:width-10], curses.color_pair(1))

            y_offset += 4

        # Footer with total
        total = sum(workout_data.values())
        footer_y = height - 5
        stdscr.addstr(footer_y, 0, "─" * width, curses.color_pair(1))

        total_str = f"TOTAL: {total}/350 reps"
        stdscr.addstr(footer_y + 1, (width - len(total_str)) // 2, total_str,
                     curses.color_pair(3) | curses.A_BOLD)

        controls = "↑/↓: Select  |  +/-: Add/Remove Reps  |  1 / 5 / 0 (+10): Quick Add  |  r: Reset  |  q: Quit"
        if len(controls) < width:
            stdscr.addstr(footer_y + 3, (width - len(controls)) // 2, controls, curses.color_pair(1))

        stdscr.refresh()

        key = stdscr.getch()

        if key == ord('q') or key == ord('Q'):
            break
        elif key == curses.KEY_UP:
            selected = (selected - 1) % len(EXERCISES)
        elif key == curses.KEY_DOWN:
            selected = (selected + 1) % len(EXERCISES)
        elif key == ord('+') or key == ord('='):
            exercise_id = EXERCISES[selected]["id"]
            workout_data[exercise_id] = workout_data.get(exercise_id, 0) + 1
            save_data(workout_data)
        elif key == ord('-') or key == ord('_'):
            exercise_id = EXERCISES[selected]["id"]
            workout_data[exercise_id] = max(0, workout_data.get(exercise_id, 0) - 1)
            save_data(workout_data)
        elif key == ord('1'):
            exercise_id = EXERCISES[selected]["id"]
            workout_data[exercise_id] = workout_data.get(exercise_id, 0) + 1
            save_data(workout_data)
        elif key == ord('5'):
            exercise_id = EXERCISES[selected]["id"]
            workout_data[exercise_id] = workout_data.get(exercise_id, 0) + 5
            save_data(workout_data)
        elif key == ord('0'):
            exercise_id = EXERCISES[selected]["id"]
            workout_data[exercise_id] = workout_data.get(exercise_id, 0) + 10
            save_data(workout_data)
        elif key == ord('r') or key == ord('R'):
            stdscr.addstr(height // 2, (width - 40) // 2,
                         "Reset all exercises? (y/n)",
                         curses.color_pair(4) | curses.A_BOLD)
            stdscr.refresh()
            confirm = stdscr.getch()
            if confirm == ord('y') or confirm == ord('Y'):
                workout_data = {ex["id"]: 0 for ex in EXERCISES}
                save_data(workout_data)


if __name__ == "__main__":
    try:
        curses.wrapper(main)
    except KeyboardInterrupt:
        pass

import { Component, OnInit } from '@angular/core';
import { StoreService } from 'src/app/services/store.service';

@Component({
  selector: 'app-date-picker',
  templateUrl: './date-picker.component.html',
  styleUrls: ['./date-picker.component.scss'],
})
export class DatePickerComponent implements OnInit {
  MONTH_NAMES = [
    { month: 'January', shortName: 'Jan' },
    { month: 'February', shortName: 'Feb' },
    { month: 'March', shortName: 'Mar' },
    { month: 'April', shortName: 'Apr' },
    { month: 'May', shortName: 'May' },
    { month: 'June', shortName: 'Jun' },
    { month: 'July', shortName: 'Jul' },
    { month: 'August', shortName: 'Aug' },
    { month: 'September', shortName: 'Sep' },
    { month: 'October', shortName: 'Oct' },
    { month: 'Novemeber', shortName: 'Nov' },
    { month: 'December', shortName: 'Dec' },
  ];
  showDatepicker = false;
  datepickerValue: string;
  selectedMonth: number;
  selectedYear: number;
  showTodayButton = false;

  constructor(public store: StoreService) {}

  ngOnInit(): void {
    this.initDate();
  }

  setSelectedMonthKey() {
    this.store.selectedMonth = `${this.selectedYear}-${this.selectedMonth}`;
  }

  initDate() {
    const today = new Date();
    this.selectedMonth = today.getMonth();
    this.selectedYear = today.getFullYear();
    this.setSelectedMonthKey();
  }

  isToday() {
    const today = new Date();
    const selectedDate = new Date(today);
    selectedDate.setMonth(this.selectedMonth);
    selectedDate.setFullYear(this.selectedYear);
    this.showTodayButton = !(today.toDateString() === selectedDate.toDateString());
  }

  selectYear(add: number) {
    this.selectedYear += add;
    this.isToday();
    this.setSelectedMonthKey();
  }

  changeMonth(add: number) {
    const newMonth = this.selectedMonth + add;
    const newDate = new Date(this.selectedYear, newMonth);
    this.selectedYear = newDate.getFullYear();
    this.selectedMonth = newDate.getMonth();
    this.isToday();
    this.setSelectedMonthKey();
  }

  selectMonth(month: number) {
    this.selectedMonth = month;
    this.isToday();
    this.setSelectedMonthKey();
    // @TODO
    // 1. select month and show its data
  }

  selectToday() {
    const date = new Date();
    this.selectedMonth = date.getMonth();
    this.selectedYear = date.getFullYear();
    this.isToday();
    this.setSelectedMonthKey();
  }
}

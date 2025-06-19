import { Pipe, PipeTransform } from '@angular/core';

@Pipe({
  name: 'addZeroPrefixToDate',
  standalone: false,
  pure: true,
})
export class AddZeroPrefixToDate implements PipeTransform {
  transform(value: string) {
    return value
      .split('-')
      .map((val) => String(val).padStart(2, '0'))
      .join('-');
  }
}

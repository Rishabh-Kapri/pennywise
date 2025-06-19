import { Pipe, PipeTransform } from "@angular/core";

@Pipe({
  name: 'abs',
  standalone: false,
})
export class AbsolutePipe implements PipeTransform {
  transform(value: number) {
    return Math.abs(value);
  }
}

package com.smartagriculture.device.repository;

import com.smartagriculture.device.domain.DeviceBinding;
import java.util.List;
import java.util.Optional;
import org.springframework.data.jpa.repository.JpaRepository;

public interface DeviceBindingRepository extends JpaRepository<DeviceBinding, Long> {

    Optional<DeviceBinding> findFirstByDeviceIdAndUnboundAtIsNull(Long deviceId);

    List<DeviceBinding> findByPlotIdAndUnboundAtIsNull(Long plotId);
}
